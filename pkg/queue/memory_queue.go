package queue

import (
	"context"
	"sync"
	"time"
)

// MemoryQueue provides a high-throughput, in-memory JobQueue implementation for testing and standalone nodes.
type MemoryQueue struct {
	mu     sync.RWMutex
	ch     chan *SubmissionJob
	closed bool
}

// NewMemoryQueue creates a new in-memory job queue with the given capacity.
func NewMemoryQueue(capacity int) *MemoryQueue {
	if capacity <= 0 {
		capacity = 10000
	}
	return &MemoryQueue{
		ch: make(chan *SubmissionJob, capacity),
	}
}

// Enqueue adds a submission job to the in-memory queue.
func (m *MemoryQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrQueueClosed
	}
	m.mu.RUnlock()

	job.Normalize()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.ch <- job:
		return nil
	default:
		return ErrBackpressureExceeded
	}
}

// Dequeue retrieves a job from the queue with timeout.
func (m *MemoryQueue) Dequeue(ctx context.Context, timeout time.Duration) (*SubmissionJob, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrQueueEmpty
	case job, ok := <-m.ch:
		if !ok {
			return nil, ErrQueueClosed
		}
		return job, nil
	}
}

// QueueDepth returns the current number of pending items in the channel.
func (m *MemoryQueue) QueueDepth(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.ch)), nil
}

// Close closes the underlying channel.
func (m *MemoryQueue) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.ch)
	}
	return nil
}

var _ JobQueue = (*MemoryQueue)(nil)
