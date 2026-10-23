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

// BullMQAdapter handles Bull and BullMQ format queues.
type BullMQAdapter struct {
	Prefix string
}

func NewBullMQAdapter(prefix string) *BullMQAdapter {
	if prefix == "" {
		prefix = "bull"
	}
	return &BullMQAdapter{Prefix: prefix}
}

func (b *BullMQAdapter) Type() QueueType {
	return TypeBullMQ
}

func (b *BullMQAdapter) key(queue, suffix string) string {
	return fmt.Sprintf("%s:%s:%s", b.Prefix, queue, suffix)
}

func (b *BullMQAdapter) jobKey(queue, id string) string {
	return fmt.Sprintf("%s:%s:%s", b.Prefix, queue, id)
}

func (b *BullMQAdapter) lockKey(queue, id string) string {
	return fmt.Sprintf("%s:%s:%s:lock", b.Prefix, queue, id)
}

// DiscoverQueues finds all Bull / BullMQ queues by scanning for keys.
func (b *BullMQAdapter) DiscoverQueues(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error) {
	pattern := fmt.Sprintf("%s:*:*", b.Prefix)
	keys, err := client.ScanKeys(ctx, rdb, pattern, 2000)
	if err != nil {
		return nil, err
	}

	queueNames := make(map[string]bool)
	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) >= 3 && parts[0] == b.Prefix {
			qName := parts[1]
			// Avoid treating job id hashes as queue names
			if qName != "" && !strings.Contains(qName, "events") {
				queueNames[qName] = true
			}
		}
	}

	var results []QueueInfo
	for qName := range queueNames {
		stats, err := b.GetQueueStats(ctx, rdb, qName)
		if err != nil {
			continue
		}
		// If total is 0, verify if any key for this queue exists
		if stats.Total == 0 {
			exists, _ := rdb.Exists(ctx, b.key(qName, "meta"), b.key(qName, "wait")).Result()
			if exists == 0 {
				continue
			}
		}

		paused, _ := b.IsQueuePaused(ctx, rdb, qName)
		results = append(results, QueueInfo{
			Name:      qName,
			Type:      b.Type(),
			Stats:     stats,
			KeyPrefix: fmt.Sprintf("%s:%s", b.Prefix, qName),
			Workers:   int(stats.Active),
			Paused:    paused,
		})
	}

	return results, nil
}

// GetQueueStats returns the count of jobs in each BullMQ bucket.
func (b *BullMQAdapter) GetQueueStats(ctx context.Context, rdb redis.UniversalClient, queue string) (QueueStats, error) {
	var stats QueueStats

	// Bull uses both Lists and Sorted Sets depending on version
	stats.Waiting = b.countKey(ctx, rdb, b.key(queue, "wait"))
	stats.Active = b.countKey(ctx, rdb, b.key(queue, "active"))
	stats.Completed = b.countKey(ctx, rdb, b.key(queue, "completed"))
	stats.Failed = b.countKey(ctx, rdb, b.key(queue, "failed"))
	stats.Delayed = b.countKey(ctx, rdb, b.key(queue, "delayed"))

	stats.Total = stats.Waiting + stats.Active + stats.Completed + stats.Failed + stats.Delayed
	return stats, nil
}

func (b *BullMQAdapter) countKey(ctx context.Context, rdb redis.UniversalClient, key string) int64 {
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
	case "set":
		n, _ := rdb.SCard(ctx, key).Result()
		return n
	}
	return 0
}

func (b *BullMQAdapter) stateKey(queue string, state JobState) string {
	suffix := string(state)
	if state == StateWaiting {
		suffix = "wait"
	}
	return b.key(queue, suffix)
}

func (b *BullMQAdapter) getJobIDs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]string, int64, error) {
	key := b.stateKey(queue, state)
	kType, err := rdb.Type(ctx, key).Result()
	if err != nil || kType == "none" {
		return nil, 0, nil
	}

	var ids []string
	var total int64

	switch kType {
	case "list":
		total, _ = rdb.LLen(ctx, key).Result()
		end := offset + limit - 1
		ids, err = rdb.LRange(ctx, key, offset, end).Result()
	case "zset":
		total, _ = rdb.ZCard(ctx, key).Result()
		// Retrieve latest items first (rev range)
		ids, err = rdb.ZRevRange(ctx, key, offset, offset+limit-1).Result()
	}

	return ids, total, err
}

// ListJobs fetches jobs for a queue in a specific state.
func (b *BullMQAdapter) ListJobs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]Job, int64, error) {
	ids, total, err := b.getJobIDs(ctx, rdb, queue, state, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	var jobs []Job
	for _, id := range ids {
		job, err := b.GetJob(ctx, rdb, queue, id)
		if err == nil && job != nil {
			job.State = state
			jobs = append(jobs, *job)
		}
	}

	return jobs, total, nil
}

// GetJob reads a single job's hash data from Redis.
func (b *BullMQAdapter) GetJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) (*Job, error) {
	jobKey := b.jobKey(queue, id)
	fields, err := rdb.HGetAll(ctx, jobKey).Result()
	if err != nil || len(fields) == 0 {
		return nil, fmt.Errorf("job %s not found in queue %s", id, queue)
	}

	job := &Job{
		ID:        id,
		Queue:     queue,
		QueueType: b.Type(),
		Name:      fields["name"],
	}

	// Payload
	rawPayload := fields["data"]
	job.RawPayload = rawPayload
	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(rawPayload), &payloadMap); err == nil {
		job.Payload = payloadMap
	} else if rawPayload != "" {
		job.Payload = map[string]interface{}{"value": rawPayload}
	} else {
		job.Payload = make(map[string]interface{})
	}

	// Timestamps
	if ts, ok := fields["timestamp"]; ok {
		if ms, err := strconv.ParseInt(ts, 10, 64); err == nil {
			job.EnqueuedAt = time.UnixMilli(ms)
		}
	}
	if proc, ok := fields["processedOn"]; ok {
		if ms, err := strconv.ParseInt(proc, 10, 64); err == nil && ms > 0 {
			t := time.UnixMilli(ms)
			job.StartedAt = &t
			if !job.EnqueuedAt.IsZero() {
				job.WaitTimeMs = t.Sub(job.EnqueuedAt).Milliseconds()
			}
		}
	}
	if fin, ok := fields["finishedOn"]; ok {
		if ms, err := strconv.ParseInt(fin, 10, 64); err == nil && ms > 0 {
			t := time.UnixMilli(ms)
			job.FinishedAt = &t
			if job.StartedAt != nil {
				job.DurationMs = t.Sub(*job.StartedAt).Milliseconds()
			}
		}
	}

	// Error & stacktrace
	job.ErrorReason = fields["failedReason"]
	if st, ok := fields["stacktrace"]; ok {
		job.StackTrace = st
	}

	// Attempts
	if att, ok := fields["attemptsMade"]; ok {
		job.Attempts, _ = strconv.Atoi(att)
	}

	// Options
	if optsStr, ok := fields["opts"]; ok {
		var opts map[string]interface{}
		if err := json.Unmarshal([]byte(optsStr), &opts); err == nil {
			if maxAtt, ok := opts["attempts"].(float64); ok {
				job.MaxAttempts = int(maxAtt)
			}
		}
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 1
	}

	// Lock key inspection
	lockKey := b.lockKey(queue, id)
	ttl, err := rdb.PTTL(ctx, lockKey).Result()
	if err == nil && ttl > 0 {
		job.LockKey = lockKey
		job.LockTTLMs = ttl.Milliseconds()
		workerID, _ := rdb.Get(ctx, lockKey).Result()
		job.WorkerID = workerID
	}

	return job, nil
}

// RetryJob removes a job from failed set and re-inserts it into wait list.
func (b *BullMQAdapter) RetryJob(ctx context.Context, rdb redis.UniversalClient, queue, id string, mutatedPayload []byte) error {
	failedKey := b.key(queue, "failed")
	waitKey := b.key(queue, "wait")
	jobKey := b.jobKey(queue, id)

	// Remove from failed
	rdb.ZRem(ctx, failedKey, id)
	rdb.LRem(ctx, failedKey, 0, id)

	// If mutatedPayload provided, update the job hash
	pipe := rdb.Pipeline()
	if len(mutatedPayload) > 0 {
		pipe.HSet(ctx, jobKey, "data", string(mutatedPayload))
	}
	pipe.HDel(ctx, jobKey, "failedReason", "stacktrace", "finishedOn")
	pipe.HSet(ctx, jobKey, "timestamp", time.Now().UnixMilli())

	// Push to wait list
	pipe.RPush(ctx, waitKey, id)

	_, err := pipe.Exec(ctx)
	return err
}

// DeleteJob completely removes a job from all states and hash keys.
func (b *BullMQAdapter) DeleteJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) error {
	pipe := rdb.Pipeline()
	pipe.LRem(ctx, b.key(queue, "wait"), 0, id)
	pipe.ZRem(ctx, b.key(queue, "wait"), id)
	pipe.LRem(ctx, b.key(queue, "active"), 0, id)
	pipe.ZRem(ctx, b.key(queue, "active"), id)
	pipe.ZRem(ctx, b.key(queue, "completed"), id)
	pipe.ZRem(ctx, b.key(queue, "failed"), id)
	pipe.ZRem(ctx, b.key(queue, "delayed"), id)
	pipe.Del(ctx, b.jobKey(queue, id))
	pipe.Del(ctx, b.lockKey(queue, id))
	_, err := pipe.Exec(ctx)
	return err
}

// PurgeQueue removes all jobs in a specified state.
func (b *BullMQAdapter) PurgeQueue(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState) (int64, error) {
	key := b.stateKey(queue, state)
	count := b.countKey(ctx, rdb, key)
	err := rdb.Del(ctx, key).Err()
	return count, err
}

// DetectStalls checks active jobs whose lock key expired or is missing.
func (b *BullMQAdapter) DetectStalls(ctx context.Context, rdb redis.UniversalClient, queue string) ([]StallWarning, error) {
	ids, _, err := b.getJobIDs(ctx, rdb, queue, StateActive, 0, 500)
	if err != nil {
		return nil, err
	}

	var warnings []StallWarning
	now := time.Now()

	for _, id := range ids {
		lockKey := b.lockKey(queue, id)
		ttl, _ := rdb.PTTL(ctx, lockKey).Result()
		workerID, _ := rdb.Get(ctx, lockKey).Result()

		job, err := b.GetJob(ctx, rdb, queue, id)
		var activeSince time.Time
		if err == nil && job.StartedAt != nil {
			activeSince = *job.StartedAt
		} else {
			activeSince = now
		}

		// A stall occurs if the lock key does not exist or has expired (< 0)
		if ttl <= 0 {
			warnings = append(warnings, StallWarning{
				JobID:       id,
				Queue:       queue,
				QueueType:   b.Type(),
				LockKey:     lockKey,
				LockTTLMs:   0,
				ActiveSince: activeSince,
				ElapsedMs:   now.Sub(activeSince).Milliseconds(),
				WorkerID:    workerID,
				Reason:      "Worker lock expired or missing while job remains active",
			})
		}
	}

	return warnings, nil
}

// ReleaseLock frees a lock key to unlock a stalled task.
func (b *BullMQAdapter) ReleaseLock(ctx context.Context, rdb redis.UniversalClient, queue, lockKey string) error {
	return rdb.Del(ctx, lockKey).Err()
}

// PauseQueue marks a BullMQ queue as paused.
func (b *BullMQAdapter) PauseQueue(ctx context.Context, rdb redis.UniversalClient, queue string) error {
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, b.key(queue, "meta"), "paused", "1")
	pipe.Set(ctx, b.key(queue, "paused"), "1", 0)
	_, err := pipe.Exec(ctx)
	return err
}

// ResumeQueue unpauses a BullMQ queue.
func (b *BullMQAdapter) ResumeQueue(ctx context.Context, rdb redis.UniversalClient, queue string) error {
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, b.key(queue, "meta"), "paused", "0")
	pipe.Del(ctx, b.key(queue, "paused"))
	_, err := pipe.Exec(ctx)
	return err
}

// IsQueuePaused checks whether the BullMQ queue is in paused state.
func (b *BullMQAdapter) IsQueuePaused(ctx context.Context, rdb redis.UniversalClient, queue string) (bool, error) {
	val, err := rdb.HGet(ctx, b.key(queue, "meta"), "paused").Result()
	if err == nil && val == "1" {
		return true, nil
	}
	exists, _ := rdb.Exists(ctx, b.key(queue, "paused")).Result()
	return exists > 0, nil
}

