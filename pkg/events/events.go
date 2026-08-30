package events

import (
	"context"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

// EventStatus represents the real-time progression state of a submission.
type EventStatus string

const (
	StatusQueued         EventStatus = "QUEUED"
	StatusCompiling      EventStatus = "COMPILING"
	StatusRunning        EventStatus = "RUNNING"
	StatusTestCaseStart  EventStatus = "TESTCASE_START"
	StatusTestCasePassed EventStatus = "TESTCASE_PASSED"
	StatusTestCaseFailed EventStatus = "TESTCASE_FAILED"
	StatusCompleted      EventStatus = "COMPLETED"
	StatusFailed         EventStatus = "FAILED"
)

// TestCaseResult captures the evaluation output of a single testcase.
type TestCaseResult struct {
	TestCaseID   string         `json:"test_case_id"`
	Index        int            `json:"index"`
	Verdict      models.Verdict `json:"verdict"`
	ExitCode     int            `json:"exit_code"`
	Stdout       string         `json:"stdout,omitempty"`
	Stderr       string         `json:"stderr,omitempty"`
	WallTimeMs   float64        `json:"wall_time_ms"`
	CpuTimeMs    float64        `json:"cpu_time_ms"`
	PeakMemoryMB float64        `json:"peak_memory_mb"`
	OOMKilled    bool           `json:"oom_killed"`
}

// ExecutionEvent is the real-time event frame streamed over WebSockets and the event bus.
type ExecutionEvent struct {
	SubmissionID    string                    `json:"submission_id"`
	Status          EventStatus               `json:"status"`
	Timestamp       time.Time                 `json:"timestamp"`
	CurrentTestCase int                       `json:"current_test_case,omitempty"`
	TotalTestCases  int                       `json:"total_test_cases,omitempty"`
	Verdict         models.Verdict            `json:"verdict,omitempty"`
	TestCaseResult  *TestCaseResult           `json:"test_case_result,omitempty"`
	Compilation     *models.CompilationResult `json:"compilation,omitempty"`
	StdoutChunk     string                    `json:"stdout_chunk,omitempty"`
	StderrChunk     string                    `json:"stderr_chunk,omitempty"`
	WallTimeMs      float64                   `json:"wall_time_ms,omitempty"`
	CpuTimeMs       float64                   `json:"cpu_time_ms,omitempty"`
	PeakMemoryMB    float64                   `json:"peak_memory_mb,omitempty"`
	ErrorDetails    string                    `json:"error_details,omitempty"`
}

// EventBus defines the contract for broadcasting real-time execution events.
type EventBus interface {
	// Publish broadcasts an event for a specific submission ID.
	Publish(ctx context.Context, submissionID string, event *ExecutionEvent) error

	// Subscribe listens for events targeting a specific submission ID. Returns a channel and a cancel function.
	Subscribe(ctx context.Context, submissionID string) (<-chan *ExecutionEvent, func(), error)

	// Close shuts down event bus connections.
	Close() error
}
