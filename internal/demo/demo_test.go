package demo_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/demo"
	"github.com/alexandrmotologa/queuelens/internal/engine"
)

func TestDemoServer(t *testing.T) {
	broadcaster := engine.NewEventBroadcaster(100)
	metrics := engine.NewMetricsEngine(60)

	ds, err := demo.StartDemoServer(broadcaster, metrics)
	if err != nil {
		t.Fatalf("failed to start demo server: %v", err)
	}
	defer ds.Close()

	if ds.Addr() == "" {
		t.Fatal("expected non-empty demo address")
	}

	ctx := context.Background()
	registry := adapters.NewRegistry()
	queues, err := registry.DiscoverAll(ctx, ds.Client)
	if err != nil {
		t.Fatalf("failed to discover demo queues: %v", err)
	}

	if len(queues) == 0 {
		t.Fatal("expected discovered demo queues, got 0")
	}

	// Verify we have multi-framework queues
	typesFound := make(map[adapters.QueueType]bool)
	for _, q := range queues {
		typesFound[q.Type] = true
	}

	if !typesFound[adapters.TypeBullMQ] {
		t.Error("expected BullMQ queues in demo")
	}
	if !typesFound[adapters.TypeAsynq] {
		t.Error("expected Asynq queues in demo")
	}
	if !typesFound[adapters.TypeSidekiq] {
		t.Error("expected Sidekiq queues in demo")
	}

	// Give simulation 1.5s to generate at least one event
	time.Sleep(1500 * time.Millisecond)
	summary := metrics.GetSummary()
	if len(summary.History) == 0 {
		t.Error("expected metrics history")
	}
}
