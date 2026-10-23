package dlq

import (
	"context"
	"fmt"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/redis/go-redis/v9"
)

// ReplayRequest defines parameters for a mass replay operation.
type ReplayRequest struct {
	QueueType      adapters.QueueType `json:"queueType"`
	Queue          string             `json:"queue"`
	JobIDs         []string           `json:"jobIds"`
	MutatedPayload []byte             `json:"mutatedPayload,omitempty"`
	RateLimitPerSec int               `json:"rateLimitPerSec"` // 0 for unlimited, or e.g. 50 jobs/sec
}

// ReplayResult summarizes the outcome of the replay execution.
type ReplayResult struct {
	TotalRequested int           `json:"totalRequested"`
	Succeeded      int           `json:"succeeded"`
	Failed         int           `json:"failed"`
	DurationMs     int64         `json:"durationMs"`
	Errors         []string      `json:"errors,omitempty"`
}

// Replayer coordinates retry execution against adapters with rate limiting.
type Replayer struct {
	registry *adapters.Registry
	rdb      redis.UniversalClient
}

// NewReplayer instantiates a replayer engine.
func NewReplayer(registry *adapters.Registry, rdb redis.UniversalClient) *Replayer {
	return &Replayer{
		registry: registry,
		rdb:      rdb,
	}
}

// ReplayBatch executes retries for a list of job IDs.
func (r *Replayer) ReplayBatch(ctx context.Context, req ReplayRequest) (ReplayResult, error) {
	start := time.Now()
	res := ReplayResult{
		TotalRequested: len(req.JobIDs),
		Errors:         make([]string, 0),
	}

	adapter, err := r.registry.Get(req.QueueType)
	if err != nil {
		return res, fmt.Errorf("adapter error: %w", err)
	}

	var interval time.Duration
	if req.RateLimitPerSec > 0 {
		interval = time.Second / time.Duration(req.RateLimitPerSec)
	}

	for _, jobID := range req.JobIDs {
		select {
		case <-ctx.Done():
			res.DurationMs = time.Since(start).Milliseconds()
			return res, ctx.Err()
		default:
		}

		err := adapter.RetryJob(ctx, r.rdb, req.Queue, jobID, req.MutatedPayload)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("job %s: %v", jobID, err))
		} else {
			res.Succeeded++
		}

		if interval > 0 {
			time.Sleep(interval)
		}
	}

	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}
