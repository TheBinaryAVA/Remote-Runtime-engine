package events

import (
	"context"
	"sync"
	"time"
)

// MemoryEventBus implements an in-memory fan-out EventBus for testing and standalone operation.
type MemoryEventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *ExecutionEvent
	closed      bool
}

// NewMemoryEventBus creates a new in-memory event bus.
func NewMemoryEventBus() *MemoryEventBus {
	return &MemoryEventBus{
		subscribers: make(map[string][]chan *ExecutionEvent),
	}
}

// Publish sends the event to all active subscriber channels for this submission ID.
func (m *MemoryEventBus) Publish(ctx context.Context, submissionID string, event *ExecutionEvent) error {
	if event == nil {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	event.SubmissionID = submissionID

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil
	}

	subs, exists := m.subscribers[submissionID]
	if !exists {
		return nil
	}

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if consumer is too slow
		}
	}
	return nil
}

// Subscribe attaches a listener channel for the given submission ID.
func (m *MemoryEventBus) Subscribe(ctx context.Context, submissionID string) (<-chan *ExecutionEvent, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	eventCh := make(chan *ExecutionEvent, 64)
	m.subscribers[submissionID] = append(m.subscribers[submissionID], eventCh)

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()

			subs := m.subscribers[submissionID]
			newSubs := make([]chan *ExecutionEvent, 0, len(subs))
			for _, ch := range subs {
				if ch != eventCh {
					newSubs = append(newSubs, ch)
				}
			}
			if len(newSubs) == 0 {
				delete(m.subscribers, submissionID)
			} else {
				m.subscribers[submissionID] = newSubs
			}
			close(eventCh)
		})
	}

	return eventCh, cleanup, nil
}

// Close closes all active subscriber channels.
func (m *MemoryEventBus) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		for _, subs := range m.subscribers {
			for _, ch := range subs {
				close(ch)
			}
		}
		m.subscribers = make(map[string][]chan *ExecutionEvent)
	}
	return nil
}

var _ EventBus = (*MemoryEventBus)(nil)
