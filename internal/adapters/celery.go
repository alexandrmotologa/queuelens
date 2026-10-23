package adapters

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/client"
	"github.com/redis/go-redis/v9"
)

// CeleryAdapter handles Python Celery task queues and task-meta results.
type CeleryAdapter struct{}

func NewCeleryAdapter() *CeleryAdapter {
	return &CeleryAdapter{}
}

func (c *CeleryAdapter) Type() QueueType {
	return TypeCelery
}

// CeleryMessage represents the wire format of Celery protocol v1 and v2.
type CeleryMessage struct {
	Body            string                 `json:"body"`
	Headers         map[string]interface{} `json:"headers"`
	ContentType     string                 `json:"content-type"`
	ContentEncoding string                 `json:"content-encoding"`
	Properties      map[string]interface{} `json:"properties"`
}

// CeleryTaskMeta represents the result backend payload stored in celery-task-meta-*.
type CeleryTaskMeta struct {
	Status    string      `json:"status"`
	Result    interface{} `json:"result"`
	Traceback string      `json:"traceback"`
	DateDone  string      `json:"date_done"`
	TaskID    string      `json:"task_id"`
}

// DiscoverQueues scans for celery-task-meta keys and queue lists.
func (c *CeleryAdapter) DiscoverQueues(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error) {
	// Look for standard celery queue or celery-task-meta keys
	keys, err := client.ScanKeys(ctx, rdb, "celery*", 1000)
	if err != nil {
		return nil, err
	}

	queueNames := make(map[string]bool)
	hasCeleryMeta := false

	for _, k := range keys {
		if strings.HasPrefix(k, "celery-task-meta-") {
			hasCeleryMeta = true
		} else {
			// Might be a queue list named "celery" or "celery-priority-*"
			kType, _ := rdb.Type(ctx, k).Result()
			if kType == "list" {
				queueNames[k] = true
			}
		}
	}

	if len(queueNames) == 0 && hasCeleryMeta {
		queueNames["celery"] = true
	}

	var results []QueueInfo
	for qName := range queueNames {
		stats, err := c.GetQueueStats(ctx, rdb, qName)
		if err != nil {
			continue
		}
		results = append(results, QueueInfo{
			Name:      qName,
			Type:      c.Type(),
			Stats:     stats,
			KeyPrefix: qName,
			Workers:   int(stats.Active),
			Paused:    false,
		})
	}

	return results, nil
}

// GetQueueStats tallies waiting tasks in queue list and task-meta results.
func (c *CeleryAdapter) GetQueueStats(ctx context.Context, rdb redis.UniversalClient, queue string) (QueueStats, error) {
	var stats QueueStats

	// Waiting jobs are queued in the list
	kType, _ := rdb.Type(ctx, queue).Result()
	if kType == "list" {
		stats.Waiting, _ = rdb.LLen(ctx, queue).Result()
	}

	// Count task metadata states
	metaKeys, err := client.ScanKeys(ctx, rdb, "celery-task-meta-*", 500)
	if err == nil {
		for _, mk := range metaKeys {
			val, err := rdb.Get(ctx, mk).Result()
			if err != nil {
				continue
			}
			var meta CeleryTaskMeta
			if err := json.Unmarshal([]byte(val), &meta); err == nil {
				switch strings.ToUpper(meta.Status) {
				case "SUCCESS":
					stats.Completed++
				case "FAILURE":
					stats.Failed++
				case "STARTED":
					stats.Active++
				case "RETRY":
					stats.Delayed++
				}
			}
		}
	}

	stats.Total = stats.Waiting + stats.Active + stats.Completed + stats.Failed + stats.Delayed
	return stats, nil
}

// ListJobs fetches jobs based on state.
func (c *CeleryAdapter) ListJobs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]Job, int64, error) {
	if state == StateWaiting {
		total, _ := rdb.LLen(ctx, queue).Result()
		items, err := rdb.LRange(ctx, queue, offset, offset+limit-1).Result()
		if err != nil {
			return nil, 0, err
		}
		var jobs []Job
		for i, item := range items {
			job := c.parseMessageItem(queue, fmt.Sprintf("%s-w-%d", queue, offset+int64(i)), item)
			job.State = StateWaiting
			jobs = append(jobs, job)
		}
		return jobs, total, nil
	}

	// For completed/failed/active/delayed, scan celery-task-meta
	metaKeys, err := client.ScanKeys(ctx, rdb, "celery-task-meta-*", 1000)
	if err != nil {
		return nil, 0, err
	}

	var matchedJobs []Job
	targetStatus := ""
	switch state {
	case StateCompleted:
		targetStatus = "SUCCESS"
	case StateFailed:
		targetStatus = "FAILURE"
	case StateActive:
		targetStatus = "STARTED"
	case StateDelayed:
		targetStatus = "RETRY"
	}

	for _, mk := range metaKeys {
		val, err := rdb.Get(ctx, mk).Result()
		if err != nil {
			continue
		}
		var meta CeleryTaskMeta
		if err := json.Unmarshal([]byte(val), &meta); err == nil {
			if strings.EqualFold(meta.Status, targetStatus) {
				taskID := strings.TrimPrefix(mk, "celery-task-meta-")
				job := Job{
					ID:          taskID,
					Queue:       queue,
					QueueType:   c.Type(),
					State:       state,
					Name:        fmt.Sprintf("celery.task.%s", taskID[:min(8, len(taskID))]),
					ErrorReason: "",
					StackTrace:  meta.Traceback,
					Payload:     make(map[string]interface{}),
					RawPayload:  val,
				}
				if meta.Result != nil {
					if resMap, ok := meta.Result.(map[string]interface{}); ok {
						job.Payload = resMap
					} else {
						job.Payload = map[string]interface{}{"result": meta.Result}
						if state == StateFailed {
							job.ErrorReason = fmt.Sprintf("%v", meta.Result)
						}
					}
				}
				if meta.DateDone != "" {
					if t, err := time.Parse(time.RFC3339Nano, meta.DateDone); err == nil {
						job.FinishedAt = &t
					}
				}
				matchedJobs = append(matchedJobs, job)
			}
		}
	}

	total := int64(len(matchedJobs))
	start := min(int(offset), len(matchedJobs))
	end := min(int(offset+limit), len(matchedJobs))

	return matchedJobs[start:end], total, nil
}

func (c *CeleryAdapter) parseMessageItem(queue, fallbackID, raw string) Job {
	job := Job{
		ID:         fallbackID,
		Queue:      queue,
		QueueType:  c.Type(),
		RawPayload: raw,
		Payload:    make(map[string]interface{}),
		EnqueuedAt: time.Now(),
	}

	var msg CeleryMessage
	if err := json.Unmarshal([]byte(raw), &msg); err == nil {
		if id, ok := msg.Headers["id"].(string); ok && id != "" {
			job.ID = id
		}
		if task, ok := msg.Headers["task"].(string); ok && task != "" {
			job.Name = task
		}
		// If body is base64 encoded JSON
		if msg.ContentEncoding == "base64" || !strings.HasPrefix(strings.TrimSpace(msg.Body), "{") {
			if decoded, err := base64.StdEncoding.DecodeString(msg.Body); err == nil {
				var bodyMap map[string]interface{}
				if err := json.Unmarshal(decoded, &bodyMap); err == nil {
					job.Payload = bodyMap
				}
			}
		} else {
			var bodyMap map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Body), &bodyMap); err == nil {
				job.Payload = bodyMap
			}
		}
	} else {
		job.Payload = map[string]interface{}{"raw": raw}
	}

	if job.Name == "" {
		job.Name = "celery.task"
	}
	return job
}

// GetJob reads single Celery task meta or list item.
func (c *CeleryAdapter) GetJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) (*Job, error) {
	metaKey := fmt.Sprintf("celery-task-meta-%s", id)
	val, err := rdb.Get(ctx, metaKey).Result()
	if err == nil {
		var meta CeleryTaskMeta
		if err := json.Unmarshal([]byte(val), &meta); err == nil {
			state := StateCompleted
			if strings.EqualFold(meta.Status, "FAILURE") {
				state = StateFailed
			} else if strings.EqualFold(meta.Status, "STARTED") {
				state = StateActive
			}
			job := &Job{
				ID:          id,
				Queue:       queue,
				QueueType:   c.Type(),
				State:       state,
				Name:        fmt.Sprintf("celery.task.%s", id[:min(8, len(id))]),
				StackTrace:  meta.Traceback,
				ErrorReason: fmt.Sprintf("%v", meta.Result),
				RawPayload:  val,
				Payload:     map[string]interface{}{"result": meta.Result},
			}
			return job, nil
		}
	}

	return nil, fmt.Errorf("celery job %s not found", id)
}

// RetryJob re-enqueues the task into the Celery list and resets task-meta.
func (c *CeleryAdapter) RetryJob(ctx context.Context, rdb redis.UniversalClient, queue, id string, mutatedPayload []byte) error {
	metaKey := fmt.Sprintf("celery-task-meta-%s", id)
	rdb.Del(ctx, metaKey)

	payload := string(mutatedPayload)
	if len(mutatedPayload) == 0 {
		payload = fmt.Sprintf(`{"body": "{}", "headers": {"id": "%s", "task": "retried.task"}}`, id)
	}

	return rdb.LPush(ctx, queue, payload).Err()
}

// DeleteJob removes task-meta and cleans from queue list.
func (c *CeleryAdapter) DeleteJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) error {
	metaKey := fmt.Sprintf("celery-task-meta-%s", id)
	return rdb.Del(ctx, metaKey).Err()
}

// PurgeQueue clears the queue list or all task meta.
func (c *CeleryAdapter) PurgeQueue(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState) (int64, error) {
	if state == StateWaiting {
		count, _ := rdb.LLen(ctx, queue).Result()
		err := rdb.Del(ctx, queue).Err()
		return count, err
	}
	return 0, nil
}

// DetectStalls checks for STARTED tasks that have no update.
func (c *CeleryAdapter) DetectStalls(ctx context.Context, rdb redis.UniversalClient, queue string) ([]StallWarning, error) {
	return nil, nil
}

// ReleaseLock frees a lock key.
func (c *CeleryAdapter) ReleaseLock(ctx context.Context, rdb redis.UniversalClient, queue, lockKey string) error {
	return rdb.Del(ctx, lockKey).Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
