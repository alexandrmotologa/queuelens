package adapters

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Registry manages and coordinates queue adapters.
type Registry struct {
	mu       sync.RWMutex
	adapters map[QueueType]QueueAdapter
}

// NewRegistry creates a registry pre-loaded with all standard adapters.
func NewRegistry() *Registry {
	r := &Registry{
		adapters: make(map[QueueType]QueueAdapter),
	}
	r.Register(NewBullMQAdapter("bull"))
	r.Register(NewCeleryAdapter())
	r.Register(NewAsynqAdapter())
	r.Register(NewSidekiqAdapter())
	return r
}

// Register adds or replaces an adapter for a queue type.
func (r *Registry) Register(adapter QueueAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Type()] = adapter
}

// Get returns the adapter for the given queue type.
func (r *Registry) Get(qType QueueType) (QueueAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[qType]
	if !ok {
		return nil, fmt.Errorf("unsupported queue type: %s", qType)
	}
	return a, nil
}

// DiscoverAll discovers all queues across all registered adapters.
func (r *Registry) DiscoverAll(ctx context.Context, rdb redis.UniversalClient) ([]QueueInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []QueueInfo
	for _, adapter := range r.adapters {
		queues, err := adapter.DiscoverQueues(ctx, rdb)
		if err == nil {
			all = append(all, queues...)
		}
	}

	return all, nil
}
