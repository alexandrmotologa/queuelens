package dlq_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/dlq"
	"github.com/redis/go-redis/v9"
)

func TestErrorClustering(t *testing.T) {
	jobs := []adapters.Job{
		{
			ID:          "job-1",
			Queue:       "emails",
			QueueType:   adapters.TypeBullMQ,
			ErrorReason: "ConnectionRefused: dial tcp 192.168.1.15:5432: connect: connection refused for user_12345",
		},
		{
			ID:          "job-2",
			Queue:       "emails",
			QueueType:   adapters.TypeBullMQ,
			ErrorReason: "ConnectionRefused: dial tcp 10.0.0.8:5432: connect: connection refused for user_67890",
		},
		{
			ID:          "job-3",
			Queue:       "orders",
			QueueType:   adapters.TypeAsynq,
			ErrorReason: "HTTP 429 Too Many Requests: Stripe rate limit exceeded for request req_99999",
		},
	}

	clusters := dlq.ClusterJobs(jobs)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	// First cluster should be ConnectionRefused with 2 jobs
	if clusters[0].Count != 2 {
		t.Errorf("expected cluster count 2, got %d", clusters[0].Count)
	}
	if clusters[1].Count != 1 {
		t.Errorf("expected cluster count 1, got %d", clusters[1].Count)
	}
}

func TestReplayer(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer rdb.Close()

	registry := adapters.NewRegistry()
	replayer := dlq.NewReplayer(registry, rdb)

	ctx := context.Background()

	// Seed failed job in BullMQ
	jobID := "fail-1"
	qName := "notifications"
	rdb.ZAdd(ctx, "bull:notifications:failed", redis.Z{Score: 1000, Member: jobID})
	rdb.HSet(ctx, "bull:notifications:fail-1", map[string]interface{}{
		"name":         "PushNotify",
		"data":         `{"msg":"old"}`,
		"failedReason": "timeout",
	})

	req := dlq.ReplayRequest{
		QueueType:       adapters.TypeBullMQ,
		Queue:           qName,
		JobIDs:          []string{jobID},
		MutatedPayload:  []byte(`{"msg":"retried"}`),
		RateLimitPerSec: 100,
	}

	res, err := replayer.ReplayBatch(ctx, req)
	if err != nil {
		t.Fatalf("replay batch error: %v", err)
	}
	if res.Succeeded != 1 || res.Failed != 0 {
		t.Errorf("expected 1 succeeded, got %+v", res)
	}

	// Verify job moved to wait list
	waitCount, _ := rdb.LLen(ctx, "bull:notifications:wait").Result()
	if waitCount != 1 {
		t.Errorf("expected 1 job in wait list, got %d", waitCount)
	}
}
