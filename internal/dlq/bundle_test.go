package dlq_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/dlq"
	"github.com/redis/go-redis/v9"
)

func TestDLQExportAndImport(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer rdb.Close()

	registry := adapters.NewRegistry()
	ctx := context.Background()

	jobs := []adapters.Job{
		{
			ID:          "job-export-1",
			Queue:       "emails",
			QueueType:   adapters.TypeBullMQ,
			Name:        "SendMail",
			RawPayload:  `{"host":"old.internal"}`,
			ErrorReason: "Timeout",
			Attempts:    3,
			EnqueuedAt:  time.Now(),
		},
	}

	// 1. Export JSON
	jsonBytes, err := dlq.ExportJSON(jobs)
	if err != nil {
		t.Fatalf("export json error: %v", err)
	}
	if !strings.Contains(string(jsonBytes), "job-export-1") {
		t.Errorf("expected job ID in json export: %s", string(jsonBytes))
	}

	// 2. Export CSV
	csvBytes, err := dlq.ExportCSV(jobs)
	if err != nil {
		t.Fatalf("export csv error: %v", err)
	}
	if !strings.Contains(string(csvBytes), "job-export-1") {
		t.Errorf("expected job ID in csv export: %s", string(csvBytes))
	}

	// 3. Seed hash in BullMQ so import can retry it
	rdb.HSet(ctx, "bull:emails:job-export-1", map[string]interface{}{
		"name":         "SendMail",
		"data":         `{"host":"old.internal"}`,
		"failedReason": "Timeout",
	})
	rdb.ZAdd(ctx, "bull:emails:failed", redis.Z{Score: 100, Member: "job-export-1"})

	// 4. Import & Replay
	importRes, err := dlq.ImportAndReplay(ctx, registry, rdb, jsonBytes)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if importRes.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %+v", importRes)
	}

	// 5. Batch Transform
	// Place job back into failed
	rdb.LRem(ctx, "bull:emails:wait", 0, "job-export-1")
	rdb.ZAdd(ctx, "bull:emails:failed", redis.Z{Score: 200, Member: "job-export-1"})

	transformReq := dlq.BatchTransformRequest{
		QueueType:       adapters.TypeBullMQ,
		Queue:           "emails",
		JobIDs:          []string{"job-export-1"},
		FindText:        "old.internal",
		ReplaceText:     "new.internal",
		ReplayAfterSave: true,
	}

	tfRes, err := dlq.BatchTransform(ctx, registry, rdb, transformReq)
	if err != nil {
		t.Fatalf("batch transform error: %v", err)
	}
	if tfRes.Succeeded != 1 {
		t.Errorf("expected 1 succeeded in batch transform, got %+v", tfRes)
	}

	// Verify transformed payload
	val, _ := rdb.HGet(ctx, "bull:emails:job-export-1", "data").Result()
	if !strings.Contains(val, "new.internal") {
		t.Errorf("expected replaced host new.internal, got %s", val)
	}
}
