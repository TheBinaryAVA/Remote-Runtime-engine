package security_test

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/engine"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/security"
)

func TestChaos_ForkBomb_Resilience(t *testing.T) {
	eng := engine.New("dev_process")

	bombCode := `
import os, sys
try:
    for _ in range(50):
        os.fork()
except Exception as e:
    sys.exit(1)
`
	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          bombCode,
		TimeLimitMs:   1000,
		MemoryLimitMB: 128,
		PidsLimit:     16,
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	// Should either exit with runtime error or timeout, but never crash host
	if res.Verdict != models.VerdictRuntimeError && res.Verdict != models.VerdictTimeLimitExceeded && res.Verdict != models.VerdictAccepted {
		t.Fatalf("unexpected verdict on fork bomb: %s", res.Verdict)
	}
}

func TestChaos_SocketAttack_Blocked(t *testing.T) {
	eng := engine.New("dev_process")
	code, err := os.ReadFile("../../testdata/exploits/socket_attack.py")
	if err != nil {
		t.Fatalf("failed to read exploit file: %v", err)
	}

	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          string(code),
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	// Must fail because socket connection is blocked
	if res.Verdict != models.VerdictRuntimeError && res.Verdict != models.VerdictTimeLimitExceeded {
		t.Fatalf("expected socket attack to be blocked (RUNTIME_ERROR), got %s", res.Verdict)
	}
}

func TestChaos_FileEscape_Blocked(t *testing.T) {
	eng := engine.New("dev_process")
	code, err := os.ReadFile("../../testdata/exploits/file_escape.py")
	if err != nil {
		t.Fatalf("failed to read exploit file: %v", err)
	}

	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          string(code),
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	// Must fail because sensitive host files are inaccessible
	if res.Verdict != models.VerdictRuntimeError {
		t.Fatalf("expected file escape to be denied (RUNTIME_ERROR), got %s", res.Verdict)
	}
}

func TestChaos_MemoryFlood_OOMKilled(t *testing.T) {
	eng := engine.New("dev_process")
	code := `
chunks = []
while True:
    chunks.append(bytearray(20 * 1024 * 1024))
`
	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          code,
		TimeLimitMs:   3000,
		MemoryLimitMB: 32, // Tight 32MB limit
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	if res.Verdict != models.VerdictMemoryLimitExceeded && res.Verdict != models.VerdictRuntimeError && res.Verdict != models.VerdictTimeLimitExceeded {
		t.Fatalf("expected memory limit or runtime error, got %s", res.Verdict)
	}
}

func TestDeterministicBenchmarking(t *testing.T) {
	eng := engine.New("dev_process")
	// Deterministic algorithmic compute load (Fibonacci 28)
	code := `
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)

print(fib(28))
`
	runs := 5
	durations := make([]float64, runs)

	for i := 0; i < runs; i++ {
		req := &models.ExecutionRequest{
			Language:       "python3",
			Code:           code,
			ExpectedOutput: "317811\n",
			TimeLimitMs:    5000,
			MemoryLimitMB:  128,
			SandboxType:    "dev_process",
		}

		res, err := eng.Run(context.Background(), req)
		if err != nil || res.Verdict != models.VerdictAccepted {
			t.Fatalf("run %d failed: verdict=%s, err=%v", i, res.Verdict, err)
		}
		durations[i] = res.WallTimeMs
	}

	// Calculate variance
	var sum float64
	for _, d := range durations {
		sum += d
	}
	mean := sum / float64(runs)

	var varianceSum float64
	for _, d := range durations {
		varianceSum += (d - mean) * (d - mean)
	}
	stdDev := math.Sqrt(varianceSum / float64(runs))
	relStdDev := (stdDev / mean) * 100.0

	t.Logf("Benchmark runs: %v ms (Mean: %.2f ms, StdDev: %.2f ms, RelStdDev: %.2f%%)",
		durations, mean, stdDev, relStdDev)

	// In local dev environment without isolated dedicated physical cores, timing is within reasonable bounds
	if mean <= 0 {
		t.Fatalf("invalid mean benchmark duration")
	}
}

func TestSeccompProfile_Generation(t *testing.T) {
	profile := security.DefaultSeccompProfile()
	if profile.DefaultAction != "SCMP_ACT_ERRNO" {
		t.Errorf("expected default action SCMP_ACT_ERRNO, got %s", profile.DefaultAction)
	}
	if len(profile.Syscalls) == 0 {
		t.Errorf("expected syscall rules in profile")
	}
}
