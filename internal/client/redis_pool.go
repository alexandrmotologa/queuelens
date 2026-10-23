package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ServerInfo holds basic health and telemetry about the target Redis/Valkey instance.
type ServerInfo struct {
	Version          string `json:"version"`
	Mode             string `json:"mode"` // standalone, cluster, sentinel
	OS               string `json:"os"`
	ConnectedClients int64  `json:"connectedClients"`
	UsedMemoryHuman  string `json:"usedMemoryHuman"`
	TotalKeys        int64  `json:"totalKeys"`
	UptimeInSeconds  int64  `json:"uptimeInSeconds"`
}

// Config provides connection settings for Redis/Valkey.
type Config struct {
	URL         string
	ClusterMode bool
	Timeout     time.Duration
}

// NewClient parses the connection URL and returns a resilient UniversalClient.
func NewClient(cfg Config) (redis.UniversalClient, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		rawURL = "redis://127.0.0.1:6379/0"
	}

	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		// Fallback for simple host:port strings without redis:// prefix
		opts = &redis.Options{
			Addr:        rawURL,
			DialTimeout: cfg.Timeout,
		}
	} else {
		opts.DialTimeout = cfg.Timeout
	}

	if cfg.ClusterMode {
		clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:       []string{opts.Addr},
			Password:    opts.Password,
			DialTimeout: cfg.Timeout,
		})
		return clusterClient, nil
	}

	return redis.NewClient(opts), nil
}

// Ping checks if the Redis server is responsive.
func Ping(ctx context.Context, rdb redis.UniversalClient) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err()
}

// GetServerInfo retrieves key metrics from Redis INFO command.
func GetServerInfo(ctx context.Context, rdb redis.UniversalClient) (*ServerInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	infoStr, err := rdb.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch redis info: %w", err)
	}

	dbSize, _ := rdb.DBSize(ctx).Result()

	info := &ServerInfo{
		Mode:      "standalone",
		TotalKeys: dbSize,
	}

	lines := strings.Split(infoStr, "\r\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "redis_version":
			info.Version = val
		case "os":
			info.OS = val
		case "redis_mode":
			info.Mode = val
		case "connected_clients":
			fmt.Sscanf(val, "%d", &info.ConnectedClients)
		case "used_memory_human":
			info.UsedMemoryHuman = val
		case "uptime_in_seconds":
			fmt.Sscanf(val, "%d", &info.UptimeInSeconds)
		}
	}

	return info, nil
}
