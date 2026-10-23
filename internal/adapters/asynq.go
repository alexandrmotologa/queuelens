package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/client"
	"github.com/redis/go-redis/v9"
)

// AsynqAdapter handles Go Asynq task queues.
type AsynqAdapter struct{}

func NewAsynqAdapter() *AsynqAdapter {
	return &AsynqAdapter{}
}

func (a *AsynqAdapter) Type() QueueType {
	return TypeAsynq
}

func (a *AsynqAdapter) queueKey(queue, suffix string) string {
	return fmt.Sprintf("asynq:{%s}:%s", queue, suffix)
}

func (a *AsynqAdapter) taskKey(queue, id string) string {
	return fmt.Sprintf("asynq:{%s}:t:%s", queue, id)
}

func (a *AsynqAdapter) leaseKey(queue, id string) string {
	return fmt.Sprintf("asynq:{%s}:lease:%s", queue, id)
}

// DiscoverQueues finds all Asynq queues by checking asynq:queues set and scanning keyspace.
func (a *AsynqAdapter) DiscoverQueues(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error) {
	queueNames := make(map[string]bool)

	// Check canonical Asynq queues set
	known, err := rdb.SMembers(ctx, "asynq:queues").Result()
	if err == nil && len(known) > 0 {
		for _, q := range known {
			queueNames[q] = true
		}
	}

	// Also scan for any asynq:{*}:* keys
	keys, err := client.ScanKeys(ctx, rdb, "asynq:{*}:*", 1000)
	if err == nil {
		for _, k := range keys {
			// Extract queue name from asynq:{<queue>}:...
			start := strings.Index(k, "{")
			end := strings.Index(k, "}")
			if start >= 0 && end > start {
				qName := k[start+1 : end]
				if qName != "" {
					queueNames[qName] = true
				}
			}
		}
	}

	var results []QueueInfo
	for qName := range queueNames {
		stats, err := a.GetQueueStats(ctx, rdb, qName)
		if err != nil {
			continue
		}
		results = append(results, QueueInfo{
			Name:      qName,
			Type:      a.Type(),
			Stats:     stats,
			KeyPrefix: fmt.Sprintf("asynq:{%s}", qName),
			Workers:   int(stats.Active),
			Paused:    false,
		})
	}

	return results, nil
}

// GetQueueStats counts jobs across Asynq sorted sets and pending lists.
func (a *AsynqAdapter) GetQueueStats(ctx context.Context, rdb redis.UniversalClient, queue string) (QueueStats, error) {
	var stats QueueStats

	// Pending can be a list or zset depending on Asynq version
	stats.Waiting = a.countKey(ctx, rdb, a.queueKey(queue, "pending"))
	stats.Active = a.countKey(ctx, rdb, a.queueKey(queue, "active"))
	stats.Completed = a.countKey(ctx, rdb, a.queueKey(queue, "completed"))
	stats.Failed = a.countKey(ctx, rdb, a.queueKey(queue, "archived"))
	stats.Delayed = a.countKey(ctx, rdb, a.queueKey(queue, "scheduled")) + a.countKey(ctx, rdb, a.queueKey(queue, "retry"))

	stats.Total = stats.Waiting + stats.Active + stats.Completed + stats.Failed + stats.Delayed
	return stats, nil
}

func (a *AsynqAdapter) countKey(ctx context.Context, rdb redis.UniversalClient, key string) int64 {
	kType, err := rdb.Type(ctx, key).Result()
	if err != nil || kType == "none" {
		return 0
	}
	switch kType {
	case "list":
		n, _ := rdb.LLen(ctx, key).Result()
		return n
	case "zset":
		n, _ := rdb.ZCard(ctx, key).Result()
		return n
	}
	return 0
}

func (a *AsynqAdapter) getJobIDs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]string, int64, error) {
	var key string
	switch state {
	case StateWaiting:
		key = a.queueKey(queue, "pending")
	case StateActive:
		key = a.queueKey(queue, "active")
	case StateCompleted:
		key = a.queueKey(queue, "completed")
	case StateFailed:
		key = a.queueKey(queue, "archived")
	case StateDelayed:
		key = a.queueKey(queue, "retry")
	}

	kType, err := rdb.Type(ctx, key).Result()
	if err != nil || kType == "none" {
		return nil, 0, nil
	}

	var ids []string
	var total int64

	switch kType {
	case "list":
		total, _ = rdb.LLen(ctx, key).Result()
		ids, err = rdb.LRange(ctx, key, offset, offset+limit-1).Result()
	case "zset":
		total, _ = rdb.ZCard(ctx, key).Result()
		ids, err = rdb.ZRevRange(ctx, key, offset, offset+limit-1).Result()
	}

	return ids, total, err
}

// ListJobs fetches tasks for a given queue and state.
func (a *AsynqAdapter) ListJobs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]Job, int64, error) {
	ids, total, err := a.getJobIDs(ctx, rdb, queue, state, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var jobs []Job
	for _, id := range ids {
		job, err := a.GetJob(ctx, rdb, queue, id)
		if err == nil && job != nil {
			job.State = state
			jobs = append(jobs, *job)
		}
	}

	return jobs, total, nil
}

// AsynqMessage represents the JSON inside the "msg" field of asynq:{q}:t:<id>.
type AsynqMessage struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Queue     string                 `json:"queue"`
	Retry     int                    `json:"retry"`
	Retried   int                    `json:"retried"`
	Timeout   int64                  `json:"timeout"`
	Deadline  int64                  `json:"deadline"`
	Completed int64                  `json:"completed_at"`
}

// GetJob reads the task hash from asynq:{q}:t:<id>.
func (a *AsynqAdapter) GetJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) (*Job, error) {
	tKey := a.taskKey(queue, id)
	fields, err := rdb.HGetAll(ctx, tKey).Result()
	if err != nil || len(fields) == 0 {
		return nil, fmt.Errorf("asynq task %s not found in queue %s", id, queue)
	}

	job := &Job{
		ID:        id,
		Queue:     queue,
		QueueType: a.Type(),
		Name:      "asynq.task",
	}

	// Parse msg JSON
	rawMsg := fields["msg"]
	job.RawPayload = rawMsg
	var msg AsynqMessage
	if err := json.Unmarshal([]byte(rawMsg), &msg); err == nil {
		if msg.Type != "" {
			job.Name = msg.Type
		}
		job.Payload = msg.Payload
		job.Attempts = msg.Retried
		job.MaxAttempts = msg.Retry
	}

	// State mapping
	switch fields["state"] {
	case "pending":
		job.State = StateWaiting
	case "active":
		job.State = StateActive
	case "completed":
		job.State = StateCompleted
	case "archived":
		job.State = StateFailed
	case "retry", "scheduled":
		job.State = StateDelayed
	}

	// Timestamps
	if ps, ok := fields["pending_since"]; ok {
		if nano, err := strconv.ParseInt(ps, 10, 64); err == nil {
			job.EnqueuedAt = time.Unix(0, nano)
		}
	}
	if lfa, ok := fields["last_failed_at"]; ok {
		if nano, err := strconv.ParseInt(lfa, 10, 64); err == nil {
			t := time.Unix(0, nano)
			job.FinishedAt = &t
		}
	}

	job.ErrorReason = fields["error_message"]

	// Inspect lease key for active task
	leaseKey := a.leaseKey(queue, id)
	ttl, err := rdb.PTTL(ctx, leaseKey).Result()
	if err == nil && ttl > 0 {
		job.LockKey = leaseKey
		job.LockTTLMs = ttl.Milliseconds()
	}

	return job, nil
}

// RetryJob moves task from archived to pending.
func (a *AsynqAdapter) RetryJob(ctx context.Context, rdb redis.UniversalClient, queue, id string, mutatedPayload []byte) error {
	archivedKey := a.queueKey(queue, "archived")
	pendingKey := a.queueKey(queue, "pending")
	tKey := a.taskKey(queue, id)

	rdb.ZRem(ctx, archivedKey, id)

	pipe := rdb.Pipeline()
	pipe.HSet(ctx, tKey, "state", "pending")
	pipe.HDel(ctx, tKey, "error_message")

	if len(mutatedPayload) > 0 {
		var msg AsynqMessage
		raw, _ := rdb.HGet(ctx, tKey, "msg").Result()
		if err := json.Unmarshal([]byte(raw), &msg); err == nil {
			var newPayload map[string]interface{}
			if err := json.Unmarshal(mutatedPayload, &newPayload); err == nil {
				msg.Payload = newPayload
				newMsgBytes, _ := json.Marshal(msg)
				pipe.HSet(ctx, tKey, "msg", string(newMsgBytes))
			}
		}
	}

	// Add to pending set/list
	pipe.RPush(ctx, pendingKey, id)
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteJob removes task from all Asynq sets and hashes.
func (a *AsynqAdapter) DeleteJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) error {
	pipe := rdb.Pipeline()
	pipe.LRem(ctx, a.queueKey(queue, "pending"), 0, id)
	pipe.ZRem(ctx, a.queueKey(queue, "pending"), id)
	pipe.ZRem(ctx, a.queueKey(queue, "active"), id)
	pipe.ZRem(ctx, a.queueKey(queue, "scheduled"), id)
	pipe.ZRem(ctx, a.queueKey(queue, "retry"), id)
	pipe.ZRem(ctx, a.queueKey(queue, "archived"), id)
	pipe.ZRem(ctx, a.queueKey(queue, "completed"), id)
	pipe.Del(ctx, a.taskKey(queue, id))
	pipe.Del(ctx, a.leaseKey(queue, id))
	_, err := pipe.Exec(ctx)
	return err
}

// PurgeQueue removes all jobs in a given state.
func (a *AsynqAdapter) PurgeQueue(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState) (int64, error) {
	var key string
	switch state {
	case StateWaiting:
		key = a.queueKey(queue, "pending")
	case StateFailed:
		key = a.queueKey(queue, "archived")
	case StateCompleted:
		key = a.queueKey(queue, "completed")
	case StateDelayed:
		key = a.queueKey(queue, "retry")
	default:
		return 0, nil
	}

	count := a.countKey(ctx, rdb, key)
	err := rdb.Del(ctx, key).Err()
	return count, err
}

// DetectStalls checks active jobs for expired or missing lease keys.
func (a *AsynqAdapter) DetectStalls(ctx context.Context, rdb redis.UniversalClient, queue string) ([]StallWarning, error) {
	ids, _, err := a.getJobIDs(ctx, rdb, queue, StateActive, 0, 500)
	if err != nil {
		return nil, err
	}

	var warnings []StallWarning
	now := time.Now()

	for _, id := range ids {
		leaseKey := a.leaseKey(queue, id)
		ttl, _ := rdb.PTTL(ctx, leaseKey).Result()

		if ttl <= 0 {
			warnings = append(warnings, StallWarning{
				JobID:       id,
				Queue:       queue,
				QueueType:   a.Type(),
				LockKey:     leaseKey,
				LockTTLMs:   0,
				ActiveSince: now,
				ElapsedMs:   0,
				Reason:      "Asynq task lease expired while task remains in active set",
			})
		}
	}

	return warnings, nil
}

// ReleaseLock frees an expired task lease key.
func (a *AsynqAdapter) ReleaseLock(ctx context.Context, rdb redis.UniversalClient, queue, lockKey string) error {
	return rdb.Del(ctx, lockKey).Err()
}
