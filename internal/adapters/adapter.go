package adapters

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// QueueType distinguishes supported queue frameworks.
type QueueType string

const (
	TypeBullMQ  QueueType = "bullmq"
	TypeCelery  QueueType = "celery"
	TypeAsynq   QueueType = "asynq"
	TypeSidekiq QueueType = "sidekiq"
)

// JobState describes the universal normalized job state.
type JobState string

const (
	StateWaiting   JobState = "waiting"
	StateActive    JobState = "active"
	StateCompleted JobState = "completed"
	StateFailed    JobState = "failed"
	StateDelayed   JobState = "delayed"
)

// Job represents a standardized queue task item.
type Job struct {
	ID          string                 `json:"id"`
	Queue       string                 `json:"queue"`
	QueueType   QueueType              `json:"queueType"`
	State       JobState               `json:"state"`
	Name        string                 `json:"name"`
	Payload     map[string]interface{} `json:"payload"`
	RawPayload  string                 `json:"rawPayload"`
	ErrorReason string                 `json:"errorReason,omitempty"`
	StackTrace  string                 `json:"stackTrace,omitempty"`
	Attempts    int                    `json:"attempts"`
	MaxAttempts int                    `json:"maxAttempts"`
	EnqueuedAt  time.Time              `json:"enqueuedAt"`
	StartedAt   *time.Time             `json:"startedAt,omitempty"`
	FinishedAt  *time.Time             `json:"finishedAt,omitempty"`
	DurationMs  int64                  `json:"durationMs"`
	WaitTimeMs  int64                  `json:"waitTimeMs"`
	LockKey     string                 `json:"lockKey,omitempty"`
	LockTTLMs   int64                  `json:"lockTTLMs,omitempty"`
	WorkerID    string                 `json:"workerId,omitempty"`
}

// QueueStats holds count of jobs in each lifecycle bucket.
type QueueStats struct {
	Waiting   int64 `json:"waiting"`
	Active    int64 `json:"active"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
	Delayed   int64 `json:"delayed"`
	Total     int64 `json:"total"`
}

// QueueInfo summarizes queue configuration and metrics.
type QueueInfo struct {
	Name      string     `json:"name"`
	Type      QueueType  `json:"type"`
	Stats     QueueStats `json:"stats"`
	KeyPrefix string     `json:"keyPrefix"`
	Workers   int        `json:"workers"`
	Paused    bool       `json:"paused"`
}

// StallWarning flags a task that is active with an expired, missing or orphaned worker lock.
type StallWarning struct {
	JobID       string    `json:"jobId"`
	Queue       string    `json:"queue"`
	QueueType   QueueType `json:"queueType"`
	LockKey     string    `json:"lockKey"`
	LockTTLMs   int64     `json:"lockTTLMs"`
	ActiveSince time.Time `json:"activeSince"`
	ElapsedMs   int64     `json:"elapsedMs"`
	WorkerID    string    `json:"workerId"`
	Reason      string    `json:"reason"`
}

// QueueAdapter abstracts access to distinct Redis-backed task queue frameworks.
type QueueAdapter interface {
	Type() QueueType
	DiscoverQueues(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error)
	GetQueueStats(ctx context.Context, rdb redis.UniversalClient, queue string) (QueueStats, error)
	ListJobs(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState, offset, limit int64) ([]Job, int64, error)
	GetJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) (*Job, error)
	RetryJob(ctx context.Context, rdb redis.UniversalClient, queue, id string, mutatedPayload []byte) error
	DeleteJob(ctx context.Context, rdb redis.UniversalClient, queue, id string) error
	PurgeQueue(ctx context.Context, rdb redis.UniversalClient, queue string, state JobState) (int64, error)
	DetectStalls(ctx context.Context, rdb redis.UniversalClient, queue string) ([]StallWarning, error)
	ReleaseLock(ctx context.Context, rdb redis.UniversalClient, queue, lockKey string) error
	PauseQueue(ctx context.Context, rdb redis.UniversalClient, queue string) error
	ResumeQueue(ctx context.Context, rdb redis.UniversalClient, queue string) error
	IsQueuePaused(ctx context.Context, rdb redis.UniversalClient, queue string) (bool, error)
}
