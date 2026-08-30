package store

import (
	"context"
	"errors"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

var (
	// ErrNotFound is returned when a submission ID does not exist in the store.
	ErrNotFound = errors.New("submission not found")
)

// SubmissionState represents the persistent state and aggregated evaluation metrics of a submission.
type SubmissionState struct {
	SubmissionID    string                    `json:"submission_id"`
	Status          events.EventStatus        `json:"status"`
	Verdict         models.Verdict            `json:"verdict"`
	Language        string                    `json:"language"`
	TotalTestCases  int                       `json:"total_test_cases"`
	PassedTestCases int                       `json:"passed_test_cases"`
	Results         []events.TestCaseResult   `json:"results,omitempty"`
	Compilation     *models.CompilationResult `json:"compilation,omitempty"`
	PeakMemoryMB    float64                   `json:"peak_memory_mb"`
	TotalCpuTimeMs  float64                   `json:"total_cpu_time_ms"`
	TotalWallTimeMs float64                   `json:"total_wall_time_ms"`
	ErrorDetails    string                    `json:"error_details,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	CompletedAt     *time.Time                `json:"completed_at,omitempty"`
}

// StateStore defines the contract for persisting submission lifecycle state and results.
type StateStore interface {
	// SaveState persists or updates the submission state.
	SaveState(ctx context.Context, state *SubmissionState) error

	// GetState retrieves the submission state by ID.
	GetState(ctx context.Context, submissionID string) (*SubmissionState, error)

	// Close closes the underlying store connections.
	Close() error
}
