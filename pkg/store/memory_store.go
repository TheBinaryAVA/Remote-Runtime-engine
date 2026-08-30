package store

import (
	"context"
	"sync"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
)

// MemoryStore implements an in-memory StateStore for testing and standalone operations.
type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]*SubmissionState
}

// NewMemoryStore creates a new in-memory state store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		states: make(map[string]*SubmissionState),
	}
}

// SaveState stores the state in memory.
func (m *MemoryStore) SaveState(ctx context.Context, state *SubmissionState) error {
	if state == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	state.UpdatedAt = time.Now()
	// Deep copy to prevent race conditions
	copied := *state
	if len(state.Results) > 0 {
		copied.Results = make([]events.TestCaseResult, len(state.Results))
		copy(copied.Results, state.Results)
	}
	m.states[state.SubmissionID] = &copied
	return nil
}

// GetState retrieves a state from memory.
func (m *MemoryStore) GetState(ctx context.Context, submissionID string) (*SubmissionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[submissionID]
	if !exists {
		return nil, ErrNotFound
	}

	copied := *state
	if len(state.Results) > 0 {
		copied.Results = make([]events.TestCaseResult, len(state.Results))
		copy(copied.Results, state.Results)
	}
	return &copied, nil
}

// Close cleans up memory store resources.
func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[string]*SubmissionState)
	return nil
}

var _ StateStore = (*MemoryStore)(nil)
