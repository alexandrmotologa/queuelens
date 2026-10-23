package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/engine"
	"github.com/redis/go-redis/v9"
)

func TestEventBroadcasterAndMetrics(t *testing.T) {
	broadcaster := engine.NewEventBroadcaster(100)
	ch := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(ch)

	metrics := engine.NewMetricsEngine(60)

	ev := engine.JobEvent{
		ID:         "test-job-1",
		Queue:      "emails",
		QueueType:  adapters.TypeBullMQ,
		Name:       "SendEmail",
		FromState:  adapters.StateWaiting,
		ToState:    adapters.StateCompleted,
		Timestamp:  time.Now(),
		DurationMs: 150,
		WaitTimeMs: 50,
	}

	broadcaster.Publish(ev)
	metrics.RecordEvent(ev)

	select {
	case received := <-ch:
		if received.ID != "test-job-1" || received.ToState != adapters.StateCompleted {
			t.Errorf("unexpected event received: %+v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}

	summary := metrics.GetSummary()
	if len(summary.History) != 60 {
		t.Errorf("expected 60 history points, got %d", len(summary.History))
	}
}

func TestStallAuditor(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer rdb.Close()

	registry := adapters.NewRegistry()
	broadcaster := engine.NewEventBroadcaster(50)
	auditor := engine.NewStallAuditor(registry, rdb, broadcaster, nil)

	ctx := context.Background()

	// Seed active job with no lock
	rdb.ZAdd(ctx, "bull:tasks:active", redis.Z{Score: 1000, Member: "stalled-job"})
	rdb.HSet(ctx, "bull:tasks:stalled-job", map[string]interface{}{
		"name": "StalledJob",
		"data": "{}",
	})

	warnings, err := auditor.AuditNow(ctx)
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}

	if len(warnings) != 1 || warnings[0].JobID != "stalled-job" {
		t.Errorf("expected 1 stall warning, got %+v", warnings)
	}
}
