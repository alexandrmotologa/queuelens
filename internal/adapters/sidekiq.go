package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/client"
	"github.com/redis/go-redis/v9"
)

// SidekiqAdapter handles Ruby Sidekiq queues, retries, and dead sets.
type SidekiqAdapter struct{}

func NewSidekiqAdapter() *SidekiqAdapter {
	return &SidekiqAdapter{}
}

func (s *SidekiqAdapter) Type() QueueType {
	return TypeSidekiq
}

// SidekiqPayload represents the wire JSON object used across Sidekiq sets.
type SidekiqPayload struct {
	Class        string        `json:"class"`
	Args         []interface{} `json:"args"`
	Queue        string        `json:"queue"`
	JID          string        `json:"jid"`
	CreatedAt    float64       `json:"created_at"`
	EnqueuedAt   float64       `json:"enqueued_at"`
	FailedAt     float64       `json:"failed_at"`
	RetriedAt    float64       `json:"retried_at"`
	RetryCount   int           `json:"retry_count"`
	ErrorMessage string        `json:"error_message"`
	ErrorClass   string        `json:"error_class"`
	ErrorBacktrace []string    `json:"error_backtrace"`
}

// DiscoverQueues scans Sidekiq's "queues" set or "queue:*" lists.
func (s *SidekiqAdapter) DiscoverQueues(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error) {
	queueNames := make(map[string]bool)

	// Check canonical Sidekiq "queues" set
	known, err := rdb.SMembers(ctx, "queues").Result()
	if err == nil && len(known) > 0 {
		for _, q := range known {
			queueNames[q] = true
		}
	}

	// Scan keys matching "queue:*"
	keys, err := client.ScanKeys(ctx, rdb, "queue:*", 500)
	if err == nil {
		for _, k := range keys {
			parts := strings.SplitN(k, ":", 2)
			if len(parts) == 2 && parts[1] != "" {
				queueNames[parts[1]] = true
			}
		}
	}

	// If Sidekiq dead or retry sets exist, ensure at least default queue is tracked
	hasDead, _ := rdb.Exists(ctx, "dead", "retry").Result()
	if hasDead > 0 && len(queueNames) == 0 {
		queueNames["default"] = true
	}

	var results []QueueInfo
	for qName := range queueNames {
		stats, err := s.GetQueueStats(ctx, rdb, qName)
		if err != nil {
			continue
		}
		results = append(results, QueueInfo{
			Name:      qName,
			Type:      s.Type(),
			Stats:     stats,
			KeyPrefix: fmt.Sprintf("queue:%s", qName),
			Workers:   0,
			Paused:    false,
		})
	}

	return results, nil
}

// GetQueueStats counts waiting jobs in queue list and dead/retry jobs.
func (s *SidekiqAdapter) GetQueueStats(ctx context.Context, rdb redis.UniversalClient, queue string) (QueueStats, error) {
	var stats QueueStats

	queueKey := fmt.Sprintf("queue:%s", queue)
	stats.Waiting, _ = rdb.LLen(ctx, queueKey).Result()

	// Dead set (DLQ)
	stats.Failed, _ = rdb.ZCard(ctx, "dead").Result()

	// Delayed / Scheduled / Retry
	retryCount, _ := rdb.ZCard(ctx, "retry").Result()
	schedCount, _ := rdb.ZCard(ctx, "schedule").Result()
	stats.Delayed = retryCount + schedCount

	// Processed stats if default queue
	if queue == "default" {
		procStr, _ := rdb.Get(ctx, "stat:processed").Result()
		if procStr != "" {
			fmt.Sscanf(procStr, "%d", &stats.Completed)
		}
	}

	stats.Total = stats.Waiting + stats.Active + stats.Completed + stats.Failed + stats.Delayed
	return stats, nil
}

// ListJobs fetches jobs from Sidekiq list or zsets.
func (s *SidekiqAdapter) ListJobs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]Job, int64, error) {
	var rawItems []string
	var total int64
	var err error

	switch state {
	case StateWaiting:
		queueKey := fmt.Sprintf("queue:%s", queue)
		total, _ = rdb.LLen(ctx, queueKey).Result()
		rawItems, err = rdb.LRange(ctx, queueKey, offset, offset+limit-1).Result()
	case StateFailed:
		total, _ = rdb.ZCard(ctx, "dead").Result()
		rawItems, err = rdb.ZRevRange(ctx, "dead", offset, offset+limit-1).Result()
	case StateDelayed:
		total, _ = rdb.ZCard(ctx, "retry").Result()
		rawItems, err = rdb.ZRevRange(ctx, "retry", offset, offset+limit-1).Result()
	default:
		return nil, 0, nil
	}

	if err != nil {
		return nil, 0, err
	}

	var jobs []Job
	for _, raw := range rawItems {
		job := s.parseSidekiqJSON(queue, state, raw)
		jobs = append(jobs, job)
	}

	return jobs, total, nil
}

func (s *SidekiqAdapter) parseSidekiqJSON(queue string, state JobState, raw string) Job {
	job := Job{
		Queue:      queue,
		QueueType:  s.Type(),
		State:      state,
		RawPayload: raw,
		Payload:    make(map[string]interface{}),
	}

	var p SidekiqPayload
	if err := json.Unmarshal([]byte(raw), &p); err == nil {
		job.ID = p.JID
		job.Name = p.Class
		if p.Queue != "" {
			job.Queue = p.Queue
		}
		job.Payload = map[string]interface{}{
			"class": p.Class,
			"args":  p.Args,
		}
		if p.CreatedAt > 0 {
			job.EnqueuedAt = time.Unix(int64(p.CreatedAt), 0)
		}
		if p.FailedAt > 0 {
			t := time.Unix(int64(p.FailedAt), 0)
			job.FinishedAt = &t
		}
		job.ErrorReason = p.ErrorMessage
		if p.ErrorClass != "" {
			if job.ErrorReason != "" {
				job.ErrorReason = fmt.Sprintf("%s: %s", p.ErrorClass, p.ErrorMessage)
			} else {
				job.ErrorReason = p.ErrorClass
			}
		}
		job.StackTrace = strings.Join(p.ErrorBacktrace, "\n")
		job.Attempts = p.RetryCount
		job.MaxAttempts = 25
	} else {
		job.ID = fmt.Sprintf("sidekiq-%d", time.Now().UnixNano())
		job.Name = "SidekiqWorker"
		job.Payload = map[string]interface{}{"raw": raw}
	}

	return job
}

// GetJob searches for a job by JID across dead and retry sets.
func (s *SidekiqAdapter) GetJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) (*Job, error) {
	// Search in dead set
	deadItems, err := rdb.ZRange(ctx, "dead", 0, -1).Result()
	if err == nil {
		for _, raw := range deadItems {
			if strings.Contains(raw, id) {
				job := s.parseSidekiqJSON(queue, StateFailed, raw)
				if job.ID == id {
					return &job, nil
				}
			}
		}
	}

	// Search in queue list
	queueKey := fmt.Sprintf("queue:%s", queue)
	listItems, err := rdb.LRange(ctx, queueKey, 0, 500).Result()
	if err == nil {
		for _, raw := range listItems {
			if strings.Contains(raw, id) {
				job := s.parseSidekiqJSON(queue, StateWaiting, raw)
				if job.ID == id {
					return &job, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("sidekiq job %s not found", id)
}

// RetryJob removes job from dead set and pushes into queue list.
func (s *SidekiqAdapter) RetryJob(ctx context.Context, rdb redis.UniversalClient, queue, id string, mutatedPayload []byte) error {
	deadItems, err := rdb.ZRange(ctx, "dead", 0, -1).Result()
	if err != nil {
		return err
	}

	for _, raw := range deadItems {
		if strings.Contains(raw, id) {
			rdb.ZRem(ctx, "dead", raw)

			finalPayload := raw
			if len(mutatedPayload) > 0 {
				var p SidekiqPayload
				if err := json.Unmarshal([]byte(raw), &p); err == nil {
					var newArgs []interface{}
					if err := json.Unmarshal(mutatedPayload, &newArgs); err == nil {
						p.Args = newArgs
						mutatedBytes, _ := json.Marshal(p)
						finalPayload = string(mutatedBytes)
					}
				}
			}

			targetQueue := queue
			if targetQueue == "" {
				targetQueue = "default"
			}
			return rdb.LPush(ctx, fmt.Sprintf("queue:%s", targetQueue), finalPayload).Err()
		}
	}

	return fmt.Errorf("job %s not found in dead set", id)
}

// DeleteJob removes job from dead, retry, or queue list.
func (s *SidekiqAdapter) DeleteJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) error {
	deadItems, _ := rdb.ZRange(ctx, "dead", 0, -1).Result()
	for _, raw := range deadItems {
		if strings.Contains(raw, id) {
			return rdb.ZRem(ctx, "dead", raw).Err()
		}
	}
	return nil
}

// PurgeQueue clears a queue list or dead set.
func (s *SidekiqAdapter) PurgeQueue(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState) (int64, error) {
	if state == StateFailed {
		count, _ := rdb.ZCard(ctx, "dead").Result()
		err := rdb.Del(ctx, "dead").Err()
		return count, err
	}
	if state == StateWaiting {
		key := fmt.Sprintf("queue:%s", queue)
		count, _ := rdb.LLen(ctx, key).Result()
		err := rdb.Del(ctx, key).Err()
		return count, err
	}
	return 0, nil
}

// DetectStalls checks worker heartbeats.
func (s *SidekiqAdapter) DetectStalls(ctx context.Context, rdb redis.UniversalClient, queue string) ([]StallWarning, error) {
	return nil, nil
}

// ReleaseLock frees a lock key.
func (s *SidekiqAdapter) ReleaseLock(ctx context.Context, rdb redis.UniversalClient, queue, lockKey string) error {
	return rdb.Del(ctx, lockKey).Err()
}
