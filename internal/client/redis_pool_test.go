package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/client"
)

func TestRedisClientAndScanner(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	cfg := client.Config{
		URL:     "redis://" + s.Addr(),
		Timeout: 2 * time.Second,
	}

	rdb, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	if err := client.Ping(ctx, rdb); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// Seed test keys
	s.Set("bull:emails:wait", "val1")
	s.Set("bull:emails:active", "val2")
	s.Set("asynq:{orders}:pending", "val3")
	s.Set("other:key", "val4")

	keys, err := client.ScanKeys(ctx, rdb, "bull:*", 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 bull:* keys, got %d: %v", len(keys), keys)
	}

	info, err := client.GetServerInfo(ctx, rdb)
	if err != nil {
		t.Fatalf("get server info failed: %v", err)
	}
	if info == nil {
		t.Fatal("expected server info, got nil")
	}
}
