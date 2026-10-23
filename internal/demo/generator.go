package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/engine"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DemoServer manages the in-memory miniredis simulation.
type DemoServer struct {
	miniServer  *miniredis.Miniredis
	Client      redis.UniversalClient
	Broadcaster *engine.EventBroadcaster
	Metrics     *engine.MetricsEngine
	cancelFunc  context.CancelFunc
}

// StartDemoServer starts an in-process Redis with pre-seeded queues and live worker simulator.
func StartDemoServer(broadcaster *engine.EventBroadcaster, metrics *engine.MetricsEngine) (*DemoServer, error) {
	s, err := miniredis.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start in-memory demo redis: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ctx, cancel := context.WithCancel(context.Background())

	ds := &DemoServer{
		miniServer:  s,
		Client:      rdb,
		Broadcaster: broadcaster,
		Metrics:     metrics,
		cancelFunc:  cancel,
	}

	ds.seedInitialData(ctx)
	ds.startLiveTrafficSimulation(ctx)

	return ds, nil
}

// Addr returns the host:port of the demo Redis server.
func (d *DemoServer) Addr() string {
	return d.miniServer.Addr()
}

// Close shuts down the simulation.
func (d *DemoServer) Close() {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}
	if d.Client != nil {
		d.Client.Close()
	}
	if d.miniServer != nil {
		d.miniServer.Close()
	}
}

func (d *DemoServer) seedInitialData(ctx context.Context) {
	now := time.Now()

	// 1. BullMQ: email-notifications
	qEmail := "email-notifications"
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("email-%04d", i)
		d.Client.RPush(ctx, fmt.Sprintf("bull:%s:wait", qEmail), id)
		d.Client.HSet(ctx, fmt.Sprintf("bull:%s:%s", qEmail, id), map[string]interface{}{
			"name":         "SendWelcomeEmail",
			"data":         fmt.Sprintf(`{"userId":"usr_%d","email":"user%d@example.com","locale":"en-US"}`, i*100, i),
			"timestamp":    now.Add(-time.Duration(i*3) * time.Second).UnixMilli(),
			"attemptsMade": "0",
			"opts":         `{"attempts":3,"backoff":{"type":"exponential","delay":1000}}`,
		})
	}

	// BullMQ: DLQ failed jobs (Error Cluster 1: SMTP Timeout)
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("fail-smtp-%02d", i)
		d.Client.ZAdd(ctx, fmt.Sprintf("bull:%s:failed", qEmail), redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: id,
		})
		d.Client.HSet(ctx, fmt.Sprintf("bull:%s:%s", qEmail, id), map[string]interface{}{
			"name":         "SendInvoiceReceipt",
			"data":         fmt.Sprintf(`{"invoiceId":"inv_%d","amount":%.2f,"currency":"USD"}`, 1000+i, float64(i)*49.99),
			"timestamp":    now.Add(-10 * time.Minute).UnixMilli(),
			"processedOn":  now.Add(-9 * time.Minute).UnixMilli(),
			"finishedOn":   now.Add(-9 * time.Minute).UnixMilli(),
			"attemptsMade": "3",
			"failedReason": fmt.Sprintf("ConnectionRefused: dial tcp 10.0.4.12:587: connect: connection timed out for invoice inv_%d", 1000+i),
			"stacktrace":   "Error: ConnectionRefused\n    at SMTPConnection._formatError (/app/node_modules/nodemailer/lib/smtp-connection/index.js:784:19)\n    at SMTPConnection._onError (/app/node_modules/nodemailer/lib/smtp-connection/index.js:770:20)",
		})
	}

	// BullMQ: Stalled active task with expired lock
	stalledID := "email-stalled-99"
	d.Client.ZAdd(ctx, fmt.Sprintf("bull:%s:active", qEmail), redis.Z{
		Score:  float64(now.Add(-15 * time.Minute).UnixMilli()),
		Member: stalledID,
	})
	d.Client.HSet(ctx, fmt.Sprintf("bull:%s:%s", qEmail, stalledID), map[string]interface{}{
		"name":         "ProcessBulkNewsletter",
		"data":         `{"campaignId":"camp_autumn_sale","recipients":25000}`,
		"timestamp":    now.Add(-20 * time.Minute).UnixMilli(),
		"processedOn":  now.Add(-15 * time.Minute).UnixMilli(),
		"attemptsMade": "1",
	})
	// Intentionally no lock key to simulate dead worker stall!

	// 2. BullMQ: image-processing
	qImg := "image-processing"
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("img-%04d", i)
		d.Client.RPush(ctx, fmt.Sprintf("bull:%s:wait", qImg), id)
		d.Client.HSet(ctx, fmt.Sprintf("bull:%s:%s", qImg, id), map[string]interface{}{
			"name":         "ResizeAvatar",
			"data":         fmt.Sprintf(`{"assetId":"ast_%04d","dimensions":[512,512],"format":"webp"}`, i),
			"timestamp":    now.Add(-time.Duration(i*5) * time.Second).UnixMilli(),
			"attemptsMade": "0",
		})
	}

	// 3. Asynq: report-generation
	qAsynq := "report-generation"
	d.Client.SAdd(ctx, "asynq:queues", qAsynq)
	for i := 1; i <= 4; i++ {
		taskID := fmt.Sprintf("asynq-task-%03d", i)
		d.Client.RPush(ctx, fmt.Sprintf("asynq:{%s}:pending", qAsynq), taskID)
		msgBytes, _ := json.Marshal(map[string]interface{}{
			"id":      taskID,
			"type":    "GenerateTaxReport",
			"payload": map[string]interface{}{"taxYear": 2026, "organizationId": fmt.Sprintf("org_%d", i)},
			"queue":   qAsynq,
			"retry":   5,
		})
		d.Client.HSet(ctx, fmt.Sprintf("asynq:{%s}:t:%s", qAsynq, taskID), map[string]interface{}{
			"msg":           string(msgBytes),
			"state":         "pending",
			"pending_since": now.UnixNano(),
		})
	}

	// Asynq DLQ (Archived) - Error Cluster 2: Database Pool Timeout
	for i := 1; i <= 3; i++ {
		taskID := fmt.Sprintf("asynq-archived-%02d", i)
		d.Client.ZAdd(ctx, fmt.Sprintf("asynq:{%s}:archived", qAsynq), redis.Z{
			Score:  float64(now.Unix()),
			Member: taskID,
		})
		msgBytes, _ := json.Marshal(map[string]interface{}{
			"id":      taskID,
			"type":    "ExportCsvAuditLog",
			"payload": map[string]interface{}{"dateRange": "2026-Q3", "entity": "security_events"},
			"queue":   qAsynq,
			"retry":   3,
			"retried": 3,
		})
		d.Client.HSet(ctx, fmt.Sprintf("asynq:{%s}:t:%s", qAsynq, taskID), map[string]interface{}{
			"msg":           string(msgBytes),
			"state":         "archived",
			"error_message": fmt.Sprintf("pgx: connection pool exhausted (timeout waiting for free connection pool_id=%d)", i*10),
			"last_failed_at": now.Add(-5 * time.Minute).UnixNano(),
		})
	}

	// 4. Sidekiq: webhook-delivery
	qSidekiq := "webhook-delivery"
	d.Client.SAdd(ctx, "queues", qSidekiq)
	for i := 1; i <= 5; i++ {
		jid := fmt.Sprintf("sidekiq-jid-%04d", i)
		payloadBytes, _ := json.Marshal(map[string]interface{}{
			"class":       "StripeWebhookDispatcher",
			"args":        []interface{}{"evt_payment_succeeded", fmt.Sprintf("cus_%d", i*55)},
			"queue":       qSidekiq,
			"jid":         jid,
			"created_at":  float64(now.Unix()),
			"enqueued_at": float64(now.Unix()),
		})
		d.Client.RPush(ctx, fmt.Sprintf("queue:%s", qSidekiq), string(payloadBytes))
	}

	// Sidekiq DLQ (dead set) - Error Cluster 3: Stripe Rate Limit
	for i := 1; i <= 4; i++ {
		jid := fmt.Sprintf("sidekiq-dead-%02d", i)
		deadBytes, _ := json.Marshal(map[string]interface{}{
			"class":          "CustomerIOWorker",
			"args":           []interface{}{fmt.Sprintf("user_%d", i*200), "event_checkout"},
			"queue":          qSidekiq,
			"jid":            jid,
			"created_at":     float64(now.Add(-2 * time.Hour).Unix()),
			"failed_at":      float64(now.Add(-30 * time.Minute).Unix()),
			"retry_count":    25,
			"error_class":    "Stripe::RateLimitError",
			"error_message":  fmt.Sprintf("HTTP 429: Too many requests for account acct_%d", i),
			"error_backtrace": []string{"/app/workers/stripe_dispatcher.rb:42:in `perform'", "/gems/stripe-5.4.0/lib/stripe.rb:108"},
		})
		d.Client.ZAdd(ctx, "dead", redis.Z{
			Score:  float64(now.Unix()),
			Member: string(deadBytes),
		})
	}
}

func (d *DemoServer) startLiveTrafficSimulation(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1200 * time.Millisecond)
		defer ticker.Stop()

		jobNames := []string{"SendWelcomeEmail", "ResizeAvatar", "GenerateTaxReport", "StripeWebhookDispatcher"}
		queues := []string{"email-notifications", "image-processing", "report-generation", "webhook-delivery"}

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				idx := rand.Intn(len(jobNames))
				q := queues[idx]
				name := jobNames[idx]
				jobID := fmt.Sprintf("live-%s", uuid.New().String()[:8])

				// 1. Enqueue event
				enqEvent := engine.JobEvent{
					ID:        jobID,
					Queue:     q,
					QueueType: adapters.TypeBullMQ,
					Name:      name,
					FromState: "",
					ToState:   adapters.StateWaiting,
					Timestamp: t,
				}
				d.Broadcaster.Publish(enqEvent)
				d.Metrics.RecordEvent(enqEvent)

				// 2. Simulate worker pickup (Wait -> Active)
				waitMs := int64(150 + rand.Intn(350))
				actEvent := engine.JobEvent{
					ID:         jobID,
					Queue:      q,
					QueueType:  adapters.TypeBullMQ,
					Name:       name,
					FromState:  adapters.StateWaiting,
					ToState:    adapters.StateActive,
					Timestamp:  t.Add(time.Duration(waitMs) * time.Millisecond),
					WaitTimeMs: waitMs,
				}
				d.Broadcaster.Publish(actEvent)
				d.Metrics.RecordEvent(actEvent)

				// 3. Complete or fail (90% success, 10% failure)
				execMs := int64(200 + rand.Intn(800))
				finishTime := actEvent.Timestamp.Add(time.Duration(execMs) * time.Millisecond)

				if rand.Float64() < 0.90 {
					finEvent := engine.JobEvent{
						ID:         jobID,
						Queue:      q,
						QueueType:  adapters.TypeBullMQ,
						Name:       name,
						FromState:  adapters.StateActive,
						ToState:    adapters.StateCompleted,
						Timestamp:  finishTime,
						DurationMs: execMs,
					}
					d.Broadcaster.Publish(finEvent)
					d.Metrics.RecordEvent(finEvent)
				} else {
					errReason := "NetworkTimeout: upstream gateway did not reply within 5000ms"
					if rand.Float64() < 0.5 {
						errReason = "DatabaseError: pq: deadlock detected in transaction"
					}
					failEvent := engine.JobEvent{
						ID:          jobID,
						Queue:       q,
						QueueType:   adapters.TypeBullMQ,
						Name:        name,
						FromState:   adapters.StateActive,
						ToState:     adapters.StateFailed,
						Timestamp:   finishTime,
						DurationMs:  execMs,
						ErrorReason: errReason,
					}
					d.Broadcaster.Publish(failEvent)
					d.Metrics.RecordEvent(failEvent)
				}
			}
		}
	}()
}
