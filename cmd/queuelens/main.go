package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/client"
	"github.com/alexandrmotologa/queuelens/internal/demo"
	"github.com/alexandrmotologa/queuelens/internal/dlq"
	"github.com/alexandrmotologa/queuelens/internal/engine"
	"github.com/alexandrmotologa/queuelens/internal/server"
	"github.com/redis/go-redis/v9"
)

const version = "1.0.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("QueueLens v%s\n", version)
			return
		case "stats":
			runStatsCommand(os.Args[2:])
			return
		case "dlq":
			runDLQCommand(os.Args[2:])
			return
		case "demo":
			runServeCommand([]string{"--demo"})
			return
		case "serve":
			runServeCommand(os.Args[2:])
			return
		}
	}

	runServeCommand(os.Args[1:])
}

func runServeCommand(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	redisURL := fs.String("redis-url", getEnv("REDIS_URL", "redis://127.0.0.1:6379/0"), "Redis / Valkey connection string")
	port := fs.Int("port", 9190, "HTTP server port for Web Studio and API")
	demoMode := fs.Bool("demo", false, "Start in-process simulated Redis with sample multi-framework queues")

	_ = fs.Parse(args)

	banner := `
   ____                              __                      
  / __ \__  ______  __  _____  / /   ___  ____  _____
 / / / / / / / _ \/ / / / _ \/ /   / _ \/ __ \/ ___/
/ /_/ / /_/ /  __/ /_/ /  __/ /___/  __/ / / (__  ) 
\___\_\__,_/\___/\__,_/\___/_____/\___/_/ /_/____/   v` + version + `
`
	fmt.Println(banner)

	registry := adapters.NewRegistry()
	broadcaster := engine.NewEventBroadcaster(300)
	metrics := engine.NewMetricsEngine(60)

	var rdb redis.UniversalClient
	var demoSrv *demo.DemoServer
	targetAddr := *redisURL

	if *demoMode {
		fmt.Println(">> Launching in-process demo Redis simulation...")
		ds, err := demo.StartDemoServer(broadcaster, metrics)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting demo server: %v\n", err)
			os.Exit(1)
		}
		demoSrv = ds
		rdb = ds.Client
		targetAddr = fmt.Sprintf("in-memory (%s)", ds.Addr())
		fmt.Printf(">> Demo workloads running (BullMQ, Celery, Asynq, Sidekiq)\n")
	} else {
		fmt.Printf(">> Connecting to Redis at %s...\n", *redisURL)
		clientInstance, err := client.NewClient(client.Config{
			URL:     *redisURL,
			Timeout: 5 * time.Second,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Redis client: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := client.Ping(ctx, clientInstance); err != nil {
			cancel()
			fmt.Fprintf(os.Stderr, "Failed to ping Redis at %s: %v\n", *redisURL, err)
			fmt.Fprintln(os.Stderr, "Tip: To test without an external Redis instance, run with: queuelens --demo")
			os.Exit(1)
		}
		cancel()
		rdb = clientInstance
		fmt.Println(">> Redis connection verified.")
	}

	stallAudit := engine.NewStallAuditor(registry, rdb, broadcaster)
	stallAudit.StartBackgroundAudit(context.Background(), 10*time.Second)

	apiServer := server.NewServer(rdb, registry, broadcaster, metrics, stallAudit, targetAddr, *demoMode)
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)
	server.RegisterStaticHandlers(mux)

	listenAddr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		fmt.Printf(">> QueueLens Studio running at http://localhost:%d\n", *port)
		fmt.Printf(">> Live REST API & WebSocket feed at http://localhost:%d/api\n", *port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n>> Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	if demoSrv != nil {
		demoSrv.Close()
	} else if rdb != nil {
		_ = rdb.Close()
	}
	fmt.Println(">> QueueLens stopped.")
}

func runStatsCommand(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	redisURL := fs.String("redis-url", getEnv("REDIS_URL", "redis://127.0.0.1:6379/0"), "Redis connection string")
	_ = fs.Parse(args)

	rdb, err := client.NewClient(client.Config{URL: *redisURL, Timeout: 3 * time.Second})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %v\n", err)
		return
	}
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	registry := adapters.NewRegistry()
	queues, err := registry.DiscoverAll(ctx, rdb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Discovery error: %v\n", err)
		return
	}

	fmt.Printf("\n%-24s %-10s %-8s %-8s %-8s %-8s %-8s\n", "QUEUE", "TYPE", "WAITING", "ACTIVE", "FAILED", "DELAYED", "TOTAL")
	fmt.Println(strings.Repeat("-", 80))
	for _, q := range queues {
		fmt.Printf("%-24s %-10s %-8d %-8d %-8d %-8d %-8d\n",
			q.Name, q.Type, q.Stats.Waiting, q.Stats.Active, q.Stats.Failed, q.Stats.Delayed, q.Stats.Total)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total detected queues: %d\n\n", len(queues))
}

func runDLQCommand(args []string) {
	if len(args) == 0 || args[0] == "list" {
		fs := flag.NewFlagSet("dlq list", flag.ExitOnError)
		redisURL := fs.String("redis-url", getEnv("REDIS_URL", "redis://127.0.0.1:6379/0"), "Redis connection string")
		_ = fs.Parse(args[1:])

		rdb, err := client.NewClient(client.Config{URL: *redisURL, Timeout: 3 * time.Second})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection error: %v\n", err)
			return
		}
		defer rdb.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		registry := adapters.NewRegistry()
		queues, _ := registry.DiscoverAll(ctx, rdb)

		var allFailed []adapters.Job
		for _, q := range queues {
			if q.Stats.Failed > 0 {
				a, _ := registry.Get(q.Type)
				jobs, _, _ := a.ListJobs(ctx, rdb, q.Name, adapters.StateFailed, 0, 1000)
				allFailed = append(allFailed, jobs...)
			}
		}

		clusters := dlq.ClusterJobs(allFailed)
		fmt.Printf("\n%-12s %-8s %-20s %-38s\n", "CLUSTER ID", "COUNT", "QUEUES", "ERROR SIGNATURE")
		fmt.Println(strings.Repeat("-", 80))
		for _, c := range clusters {
			sig := c.Signature
			if len(sig) > 36 {
				sig = sig[:33] + "..."
			}
			fmt.Printf("%-12s %-8d %-20s %-38s\n", c.ID, c.Count, strings.Join(c.Queues, ","), sig)
		}
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("Total dead jobs: %d across %d distinct error clusters\n\n", len(allFailed), len(clusters))
		return
	}

	fmt.Println("Usage: queuelens dlq list [--redis-url=...]")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
