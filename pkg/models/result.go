package models

import "time"

// CompilationResult captures the outcome of compiling a source file (e.g. C++).
type CompilationResult struct {
	Success  bool    `json:"success"`
	ExitCode int     `json:"exit_code"`
	Stdout   string  `json:"stdout,omitempty"`
	Stderr   string  `json:"stderr,omitempty"`
	TimeMs   float64 `json:"time_ms"`
}

// ExecutionResult represents the complete performance metrics and verdict of a submission.
type ExecutionResult struct {
	ID             string             `json:"id"`
	Verdict        Verdict            `json:"verdict"`
	ExitCode       int                `json:"exit_code"`
	Stdout         string             `json:"stdout"`
	Stderr         string             `json:"stderr"`
	WallTimeMs     float64            `json:"wall_time_ms"`
	CpuTimeMs      float64            `json:"cpu_time_ms"`
	PeakMemoryKB   int64              `json:"peak_memory_kb"`
	PeakMemoryMB   float64            `json:"peak_memory_mb"`
	OOMKilled      bool               `json:"oom_killed"`
	Compilation    *CompilationResult `json:"compilation,omitempty"`
	ErrorDetails   string             `json:"error_details,omitempty"`
	SandboxBackend string             `json:"sandbox_backend"`
	ExecutedAt     time.Time          `json:"executed_at"`
}
