package models

// Verdict represents the final status code of a submission execution.
type Verdict string

const (
	// VerdictAccepted indicates the program compiled, executed, and produced the expected output within constraints.
	VerdictAccepted Verdict = "ACCEPTED"

	// VerdictWrongAnswer indicates the program executed successfully but output did not match expected output.
	VerdictWrongAnswer Verdict = "WRONG_ANSWER"

	// VerdictTimeLimitExceeded indicates the execution exceeded the specified wall-clock or CPU timeout.
	VerdictTimeLimitExceeded Verdict = "TIME_LIMIT_EXCEEDED"

	// VerdictMemoryLimitExceeded indicates the program exceeded the cgroup memory.max threshold (OOM).
	VerdictMemoryLimitExceeded Verdict = "MEMORY_LIMIT_EXCEEDED"

	// VerdictCompilationError indicates failure during the language compilation phase.
	VerdictCompilationError Verdict = "COMPILATION_ERROR"

	// VerdictRuntimeError indicates non-zero exit, signal termination (SIGSEGV, SIGFPE, fork-bomb failure), etc.
	VerdictRuntimeError Verdict = "RUNTIME_ERROR"

	// VerdictOutputLimitExceeded indicates the program produced excessive stdout/stderr exceeding file caps.
	VerdictOutputLimitExceeded Verdict = "OUTPUT_LIMIT_EXCEEDED"

	// VerdictSystemError indicates an internal sandbox, filesystem, or kernel orchestration error.
	VerdictSystemError Verdict = "SYSTEM_ERROR"
)

// String returns the string representation of the verdict.
func (v Verdict) String() string {
	return string(v)
}
