package queue

import (
	"context"
	"errors"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

var (
	// ErrQueueEmpty is returned when a dequeue operation times out with no available jobs.
	ErrQueueEmpty = errors.New("queue is empty")

	// ErrQueueClosed is returned when an operation is attempted on a closed queue.
	ErrQueueClosed = errors.New("queue is closed")

	// ErrBackpressureExceeded is returned when the queue depth exceeds safety threshold.
	ErrBackpressureExceeded = errors.New("queue capacity exceeded; backpressure active")
)

// TestCase represents an individual evaluation testcase for a submission.
type TestCase struct {
	ID             string `json:"id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsHidden       bool   `json:"is_hidden"`
}

// SubmissionJob represents the complete asynchronous task payload enqueued for workers.
type SubmissionJob struct {
	SubmissionID   string     `json:"submission_id"`
	Language       string     `json:"language"`
	Code           string     `json:"code"`
	TimeLimitMs    int64      `json:"time_limit_ms"`
	MemoryLimitMB  int64      `json:"memory_limit_mb"`
	CpuQuota       float64    `json:"cpu_quota"`
	PidsLimit      int64      `json:"pids_limit"`
	MaxOutputBytes int64      `json:"max_output_bytes"`
	SandboxType    string     `json:"sandbox_type"`
	TestCases      []TestCase `json:"test_cases"`
	EnqueuedAt     time.Time  `json:"enqueued_at"`
}

// Normalize sets default resource bounds on the submission job if missing.
func (j *SubmissionJob) Normalize() {
	if j.TimeLimitMs <= 0 {
		j.TimeLimitMs = models.DefaultTimeLimitMs
	}
	if j.MemoryLimitMB <= 0 {
		j.MemoryLimitMB = models.DefaultMemoryLimitMB
	}
	if j.CpuQuota <= 0 {
		j.CpuQuota = models.DefaultCpuQuota
	}
	if j.PidsLimit <= 0 {
		j.PidsLimit = models.DefaultPidsLimit
	}
	if j.MaxOutputBytes <= 0 {
		j.MaxOutputBytes = models.DefaultMaxOutputBytes
	}
	if j.SandboxType == "" {
		j.SandboxType = "auto"
	}
	if j.EnqueuedAt.IsZero() {
		j.EnqueuedAt = time.Now()
	}
}

// JobQueue defines the contract for distributed submission queuing.
type JobQueue interface {
	// Enqueue pushes a submission job to the tail of the queue.
	Enqueue(ctx context.Context, job *SubmissionJob) error

	// Dequeue pulls a submission job from the head of the queue, blocking up to timeout.
	Dequeue(ctx context.Context, timeout time.Duration) (*SubmissionJob, error)

	// QueueDepth returns the current number of pending jobs in the queue.
	QueueDepth(ctx context.Context) (int64, error)

	// Close closes connections to the queue backend.
	Close() error
}
