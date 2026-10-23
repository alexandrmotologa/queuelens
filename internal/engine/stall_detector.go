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
	mu          sync.RWMutex
	warnings    []adapters.StallWarning
	lastAudit   time.Time
}

// NewStallAuditor creates a stall auditor instance.
func NewStallAuditor(registry *adapters.Registry, rdb redis.UniversalClient, broadcaster *EventBroadcaster) *StallAuditor {
	return &StallAuditor{
		registry:    registry,
		rdb:         rdb,
		broadcaster: broadcaster,
		warnings:    make([]adapters.StallWarning, 0),
	}
}

// AuditNow performs an immediate sweep across all registered queues.
func (s *StallAuditor) AuditNow(ctx context.Context) ([]adapters.StallWarning, error) {
	queues, err := s.registry.DiscoverAll(ctx, s.rdb)
	if err != nil {
		return nil, err
	}

	var allWarnings []adapters.StallWarning
	for _, q := range queues {
		adapter, err := s.registry.Get(q.Type)
		if err != nil {
			continue
		}

		warnings, err := adapter.DetectStalls(ctx, s.rdb, q.Name)
		if err == nil {
			allWarnings = append(allWarnings, warnings...)
		}
	}

	s.mu.Lock()
	s.warnings = allWarnings
	s.lastAudit = time.Now()
	s.mu.Unlock()

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
