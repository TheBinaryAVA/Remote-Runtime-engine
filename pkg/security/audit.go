package security

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// SecurityViolationType classifies the nature of a security boundary breach attempt.
type SecurityViolationType string

const (
	ViolationSyscallBlocked    SecurityViolationType = "SECCOMP_SYSCALL_BLOCKED"
	ViolationNetworkAttempt    SecurityViolationType = "NETWORK_SOCKET_DENIED"
	ViolationMemoryOOM         SecurityViolationType = "MEMORY_OOM_KILLED"
	ViolationForkBomb          SecurityViolationType = "PID_EXHAUSTION_FORK_BOMB"
	ViolationUnauthorizedFile  SecurityViolationType = "UNAUTHORIZED_FILE_ACCESS"
	ViolationTimeoutExceeded   SecurityViolationType = "WALL_CLOCK_TIMEOUT"
)

// AuditEvent represents a structured JSON security audit record.
type AuditEvent struct {
	Timestamp      time.Time             `json:"timestamp"`
	SubmissionID   string                `json:"submission_id"`
	ViolationType  SecurityViolationType `json:"violation_type"`
	Language       string                `json:"language"`
	SandboxBackend string                `json:"sandbox_backend"`
	Details        string                `json:"details"`
	ClientIP       string                `json:"client_ip,omitempty"`
	HostName       string                `json:"hostname"`
}

var auditLogger = log.New(os.Stderr, "[SECURITY AUDIT] ", 0)

// LogViolation records a structured JSON security incident log.
func LogViolation(submissionID, lang, sandbox string, vType SecurityViolationType, details string) {
	hostname, _ := os.Hostname()
	event := AuditEvent{
		Timestamp:      time.Now().UTC(),
		SubmissionID:   submissionID,
		ViolationType:  vType,
		Language:       lang,
		SandboxBackend: sandbox,
		Details:        details,
		HostName:       hostname,
	}

	data, err := json.Marshal(event)
	if err == nil {
		auditLogger.Println(string(data))
	}
}
