package engine

import (
	"sync"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
)

// JobEvent records an observable state change in the job lifecycle.
type JobEvent struct {
	ID          string             `json:"id"`
	Queue       string             `json:"queue"`
	QueueType   adapters.QueueType `json:"queueType"`
	Name        string             `json:"name"`
	FromState   adapters.JobState  `json:"fromState"`
	ToState     adapters.JobState  `json:"toState"`
	Timestamp   time.Time          `json:"timestamp"`
	DurationMs  int64              `json:"durationMs,omitempty"`
	WaitTimeMs  int64              `json:"waitTimeMs,omitempty"`
	ErrorReason string             `json:"errorReason,omitempty"`
}

// EventBroadcaster delivers job transition events to subscribers (WebSockets, metrics, etc.).
type EventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan JobEvent]struct{}
	history     []JobEvent
	maxHistory  int
}

// NewEventBroadcaster instantiates a thread-safe pub/sub engine.
func NewEventBroadcaster(maxHistory int) *EventBroadcaster {
	if maxHistory <= 0 {
		maxHistory = 200
	}
	return &EventBroadcaster{
		subscribers: make(map[chan JobEvent]struct{}),
		history:     make([]JobEvent, 0, maxHistory),
		maxHistory:  maxHistory,
	}
}

// Subscribe returns an event channel. Call Unsubscribe when finished.
func (b *EventBroadcaster) Subscribe() chan JobEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan JobEvent, 100)
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes an event channel and closes it.
func (b *EventBroadcaster) Unsubscribe(ch chan JobEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

// Publish distributes an event to all subscribers without blocking.
func (b *EventBroadcaster) Publish(event JobEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Append to recent history
	if len(b.history) >= b.maxHistory {
		b.history = b.history[1:]
	}
	b.history = append(b.history, event)

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if consumer buffer is saturated
		}
	}
}

// GetRecentHistory returns snapshot of recent transitions.
func (b *EventBroadcaster) GetRecentHistory() []JobEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	res := make([]JobEvent, len(b.history))
	copy(res, b.history)
	return res
}
