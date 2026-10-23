package adapters_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return s, rdb
}

func TestBullMQAdapter(t *testing.T) {
	s, rdb := setupTestRedis(t)
	defer s.Close()
	defer rdb.Close()

	ctx := context.Background()
	adapter := adapters.NewBullMQAdapter("bull")

	// 1. Seed BullMQ queue
	now := time.Now()
	jobID := "job-101"
	qName := "emails"

	rdb.RPush(ctx, "bull:emails:wait", jobID)
	rdb.HSet(ctx, "bull:emails:job-101", map[string]interface{}{
		"name":         "SendWelcomeEmail",
		"data":         `{"to":"user@example.com","template":"welcome"}`,
		"timestamp":    now.UnixMilli(),
		"attemptsMade": "0",
	})

	// Discover
	queues, err := adapter.DiscoverQueues(ctx, rdb)
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(queues) != 1 || queues[0].Name != "emails" {
		t.Fatalf("expected 1 queue emails, got %v", queues)
	}
	if queues[0].Stats.Waiting != 1 {
		t.Fatalf("expected 1 waiting job, got %d", queues[0].Stats.Waiting)
	}

	// ListJobs
	jobs, total, err := adapter.ListJobs(ctx, rdb, qName, adapters.StateWaiting, 0, 10)
	if err != nil {
		t.Fatalf("list jobs failed: %v", err)
	}
	if total != 1 || len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d / len %d", total, len(jobs))
	}
	if jobs[0].Name != "SendWelcomeEmail" {
		t.Errorf("expected job name SendWelcomeEmail, got %s", jobs[0].Name)
	}

	// Move to failed
	rdb.LRem(ctx, "bull:emails:wait", 0, jobID)
	rdb.ZAdd(ctx, "bull:emails:failed", redis.Z{Score: float64(now.UnixMilli()), Member: jobID})
	rdb.HSet(ctx, "bull:emails:job-101", "failedReason", "SMTP connection timeout")

	stats, _ := adapter.GetQueueStats(ctx, rdb, qName)
	if stats.Failed != 1 || stats.Waiting != 0 {
		t.Errorf("expected 1 failed, 0 waiting, got %+v", stats)
	}

	// Retry failed job with mutated payload
	mutated := []byte(`{"to":"user@example.com","template":"welcome_v2"}`)
	err = adapter.RetryJob(ctx, rdb, qName, jobID, mutated)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	// Check wait list again
	stats, _ = adapter.GetQueueStats(ctx, rdb, qName)
	if stats.Waiting != 1 || stats.Failed != 0 {
		t.Errorf("expected 1 waiting after retry, got %+v", stats)
	}
	job, _ := adapter.GetJob(ctx, rdb, qName, jobID)
	if job.Payload["template"] != "welcome_v2" {
		t.Errorf("expected mutated payload welcome_v2, got %v", job.Payload["template"])
	}

	// Stall test
	rdb.ZAdd(ctx, "bull:emails:active", redis.Z{Score: float64(now.UnixMilli()), Member: "job-stalled"})
	rdb.HSet(ctx, "bull:emails:job-stalled", map[string]interface{}{
		"name":        "StalledTask",
		"data":        `{}`,
		"processedOn": now.UnixMilli(),
	})
	// Lock key missing or expired
	stalls, err := adapter.DetectStalls(ctx, rdb, qName)
	if err != nil {
		t.Fatalf("detect stalls failed: %v", err)
	}
	if len(stalls) != 1 || stalls[0].JobID != "job-stalled" {
		t.Fatalf("expected 1 stall for job-stalled, got %v", stalls)
	}

	// Pause & Resume test
	if err := adapter.PauseQueue(ctx, rdb, qName); err != nil {
		t.Fatalf("pause queue failed: %v", err)
	}
	paused, _ := adapter.IsQueuePaused(ctx, rdb, qName)
	if !paused {
		t.Fatal("expected queue to be paused")
	}
	if err := adapter.ResumeQueue(ctx, rdb, qName); err != nil {
		t.Fatalf("resume queue failed: %v", err)
	}
	paused, _ = adapter.IsQueuePaused(ctx, rdb, qName)
	if paused {
		t.Fatal("expected queue to be unpaused")
	}
}

func TestAsynqAdapter(t *testing.T) {
	s, rdb := setupTestRedis(t)
	defer s.Close()
	defer rdb.Close()

	ctx := context.Background()
	adapter := adapters.NewAsynqAdapter()

	qName := "orders"
	taskID := "task-555"

	// Seed asynq
	rdb.SAdd(ctx, "asynq:queues", qName)
	rdb.RPush(ctx, "asynq:{orders}:pending", taskID)

	msgData := map[string]interface{}{
		"id":      taskID,
		"type":    "ProcessOrder",
		"payload": map[string]interface{}{"order_id": "ORD-123"},
		"queue":   qName,
		"retry":   3,
	}
	msgBytes, _ := json.Marshal(msgData)

	rdb.HSet(ctx, "asynq:{orders}:t:task-555", map[string]interface{}{
		"msg":           string(msgBytes),
		"state":         "pending",
		"pending_since": time.Now().UnixNano(),
	})

	queues, err := adapter.DiscoverQueues(ctx, rdb)
	if err != nil {
		t.Fatalf("asynq discover failed: %v", err)
	}
	if len(queues) != 1 || queues[0].Name != "orders" {
		t.Fatalf("expected queue orders, got %v", queues)
	}

	job, err := adapter.GetJob(ctx, rdb, qName, taskID)
	if err != nil {
		t.Fatalf("get job failed: %v", err)
	}
	if job.Name != "ProcessOrder" || job.Payload["order_id"] != "ORD-123" {
		t.Errorf("unexpected job details: %+v", job)
	}
}

func TestSidekiqAdapter(t *testing.T) {
	s, rdb := setupTestRedis(t)
	defer s.Close()
	defer rdb.Close()

	ctx := context.Background()
	adapter := adapters.NewSidekiqAdapter()

	qName := "default"
	payload := `{"class":"InvoiceWorker","args":["inv_100"],"queue":"default","jid":"jid-999","created_at":1600000000,"error_message":"Stripe API down","error_class":"Stripe::APIError"}`

	rdb.SAdd(ctx, "queues", qName)
	rdb.ZAdd(ctx, "dead", redis.Z{Score: 1600000000, Member: payload})

	queues, err := adapter.DiscoverQueues(ctx, rdb)
	if err != nil {
		t.Fatalf("sidekiq discover failed: %v", err)
	}
	if len(queues) == 0 {
		t.Fatal("expected at least default queue")
	}

	job, err := adapter.GetJob(ctx, rdb, qName, "jid-999")
	if err != nil {
		t.Fatalf("get sidekiq job failed: %v", err)
	}
	if job.ErrorReason != "Stripe::APIError: Stripe API down" {
		t.Errorf("unexpected error reason: %s", job.ErrorReason)
	}

	// Retry
	err = adapter.RetryJob(ctx, rdb, qName, "jid-999", nil)
	if err != nil {
		t.Fatalf("sidekiq retry failed: %v", err)
	}
	waiting, _ := rdb.LLen(ctx, "queue:default").Result()
	if waiting != 1 {
		t.Errorf("expected 1 job in queue:default, got %d", waiting)
	}
}
