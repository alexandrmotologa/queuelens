package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/redis/go-redis/v9"
)

// StallAuditor runs periodic checks on active jobs to flag dead workers and expired locks.
type StallAuditor struct {
	registry    *adapters.Registry
	rdb         redis.UniversalClient
	broadcaster *EventBroadcaster
	notifier    *WebhookNotifier
	mu          sync.RWMutex
	warnings    []adapters.StallWarning
	lastAudit   time.Time
}

// NewStallAuditor creates a stall auditor instance.
func NewStallAuditor(registry *adapters.Registry, rdb redis.UniversalClient, broadcaster *EventBroadcaster, notifier *WebhookNotifier) *StallAuditor {
	return &StallAuditor{
		registry:    registry,
		rdb:         rdb,
		broadcaster: broadcaster,
		notifier:    notifier,
		warnings:    make([]adapters.StallWarning, 0),
	}
}

// SetRedisClient updates the Redis client instance (e.g. on connection switch).
func (s *StallAuditor) SetRedisClient(rdb redis.UniversalClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rdb = rdb
}

// AuditNow performs an immediate sweep across all registered queues.
func (s *StallAuditor) AuditNow(ctx context.Context) ([]adapters.StallWarning, error) {
	s.mu.RLock()
	client := s.rdb
	notifier := s.notifier
	s.mu.RUnlock()

	queues, err := s.registry.DiscoverAll(ctx, client)
	if err != nil {
		return nil, err
	}

	var allWarnings []adapters.StallWarning
	for _, q := range queues {
		adapter, err := s.registry.Get(q.Type)
		if err != nil {
			continue
		}

		warnings, err := adapter.DetectStalls(ctx, client, q.Name)
		if err == nil {
			allWarnings = append(allWarnings, warnings...)
		}
	}

	s.mu.Lock()
	s.warnings = allWarnings
	s.lastAudit = time.Now()
	s.mu.Unlock()

	if len(allWarnings) > 0 && notifier != nil {
		_ = notifier.SendAlert(ctx, AlertEvent{
			Type:      "stall_detected",
			Title:     "Worker Lock Stall Detected",
			Message:   fmt.Sprintf("%d stalled worker jobs detected holding expired locks", len(allWarnings)),
			Severity:  SeverityWarning,
			Timestamp: time.Now(),
		})
	}

	return allWarnings, nil
}

// GetWarnings returns cached stall warnings from the latest audit.
func (s *StallAuditor) GetWarnings() []adapters.StallWarning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]adapters.StallWarning, len(s.warnings))
	copy(res, s.warnings)
	return res
}

// ForceUnlock releases an orphaned lock key via the corresponding adapter.
func (s *StallAuditor) ForceUnlock(ctx context.Context, qType adapters.QueueType, queue, lockKey string) error {
	adapter, err := s.registry.Get(qType)
	if err != nil {
		return fmt.Errorf("unknown queue type: %w", err)
	}

	return adapter.ReleaseLock(ctx, s.rdb, queue, lockKey)
}

// StartBackgroundAudit runs periodic sweeps every interval.
func (s *StallAuditor) StartBackgroundAudit(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.AuditNow(ctx)
			}
		}
	}()
}
