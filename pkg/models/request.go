package models

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// Default execution constraints for Phase 1.
const (
	DefaultTimeLimitMs    int64   = 2000     // 2.0 seconds wall-clock timeout
	DefaultMemoryLimitMB  int64   = 128      // 128 Megabytes cgroups v2 memory.max
	DefaultCpuQuota       float64 = 1.0      // 100% of 1 CPU core
	DefaultPidsLimit      int64   = 32       // Max 32 child processes/threads (fork bomb guard)
	DefaultMaxOutputBytes int64   = 1048576  // 1 MB maximum output capture
	MaxAllowedTimeLimitMs int64   = 10000    // 10 seconds hard upper cap
	MaxAllowedMemoryMB    int64   = 1024     // 1 GB hard upper cap
)

// ExecutionRequest represents a submission payload and its resource boundaries.
type ExecutionRequest struct {
	ID             string  `json:"id"`
	Language       string  `json:"language"`
	Code           string  `json:"code"`
	Stdin          string  `json:"stdin,omitempty"`
	ExpectedOutput string  `json:"expected_output,omitempty"`
	TimeLimitMs    int64   `json:"time_limit_ms"`
	MemoryLimitMB  int64   `json:"memory_limit_mb"`
	CpuQuota       float64 `json:"cpu_quota"`
	PidsLimit      int64   `json:"pids_limit"`
	MaxOutputBytes int64   `json:"max_output_bytes"`
	SandboxType    string  `json:"sandbox_type,omitempty"` // "auto", "native", "docker"
}

// Normalize applies defaults and sanitizes bounds on the execution request.
func (r *ExecutionRequest) Normalize() {
	if r.ID == "" {
		r.ID = generateExecutionID()
	}
	r.Language = strings.ToLower(strings.TrimSpace(r.Language))
	if r.Language == "py" || r.Language == "python" {
		r.Language = "python3"
	}
	if r.Language == "c++" || r.Language == "g++" {
		r.Language = "cpp"
	}

	if r.TimeLimitMs <= 0 {
		r.TimeLimitMs = DefaultTimeLimitMs
	} else if r.TimeLimitMs > MaxAllowedTimeLimitMs {
		r.TimeLimitMs = MaxAllowedTimeLimitMs
	}

	if r.MemoryLimitMB <= 0 {
		r.MemoryLimitMB = DefaultMemoryLimitMB
	} else if r.MemoryLimitMB > MaxAllowedMemoryMB {
		r.MemoryLimitMB = MaxAllowedMemoryMB
	}

	if r.CpuQuota <= 0 {
		r.CpuQuota = DefaultCpuQuota
	}

	if r.PidsLimit <= 0 {
		r.PidsLimit = DefaultPidsLimit
	}

	if r.MaxOutputBytes <= 0 {
		r.MaxOutputBytes = DefaultMaxOutputBytes
	}

	if r.SandboxType == "" {
		r.SandboxType = "auto"
	}
}

// TimeoutDuration returns the time.Duration of the requested time limit.
func (r *ExecutionRequest) TimeoutDuration() time.Duration {
	return time.Duration(r.TimeLimitMs) * time.Millisecond
}

func generateExecutionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "exec-" + time.Now().Format("20060102150405")
	}
	return "exec-" + hex.EncodeToString(b)
}
