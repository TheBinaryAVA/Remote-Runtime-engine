package engine_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/engine"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

func TestAcceptedExecution_Python(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:       "python3",
		Code:           "a, b = map(int, input().split())\nprint(a + b)",
		Stdin:          "10 25\n",
		ExpectedOutput: "35\n",
		TimeLimitMs:    2000,
		MemoryLimitMB:  128,
		SandboxType:    "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictAccepted {
		t.Fatalf("expected verdict ACCEPTED, got %s (stderr: %s)", res.Verdict, res.Stderr)
	}

	if strings.TrimSpace(res.Stdout) != "35" {
		t.Fatalf("expected stdout '35', got '%s'", res.Stdout)
	}

	if res.WallTimeMs <= 0 {
		t.Fatalf("expected positive wall time, got %.2f ms", res.WallTimeMs)
	}
}

func TestTimeLimitExceeded_Python(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          "import time\nwhile True:\n    time.sleep(0.05)",
		TimeLimitMs:   300, // 300ms limit
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}

	start := time.Now()
	res, err := eng.Run(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictTimeLimitExceeded {
		t.Fatalf("expected verdict TIME_LIMIT_EXCEEDED, got %s", res.Verdict)
	}

	if elapsed > 2*time.Second {
		t.Fatalf("watchdog timeout took too long: %v", elapsed)
	}
}

func TestRuntimeError_Python(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:      "python3",
		Code:          "print(10 / 0)",
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictRuntimeError {
		t.Fatalf("expected verdict RUNTIME_ERROR, got %s", res.Verdict)
	}

	if !strings.Contains(res.Stderr, "ZeroDivisionError") {
		t.Fatalf("expected ZeroDivisionError in stderr, got: %s", res.Stderr)
	}
}

func TestCompilationError_Cpp(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:      "cpp",
		Code:          "#include <iostream>\nint main() { InvalidSyntaxHere !!! return 0; }",
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictCompilationError {
		// If g++ is not installed on the system, compilation will also fail with error
		if res.Verdict != models.VerdictSystemError {
			t.Fatalf("expected COMPILATION_ERROR or SYSTEM_ERROR, got %s", res.Verdict)
		}
	}
}

func TestWrongAnswer_Python(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:       "python3",
		Code:           "print('incorrect')",
		ExpectedOutput: "correct_output",
		TimeLimitMs:    2000,
		MemoryLimitMB:  128,
		SandboxType:    "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictWrongAnswer {
		t.Fatalf("expected verdict WRONG_ANSWER, got %s", res.Verdict)
	}
}

func TestOutputLimitExceeded_Python(t *testing.T) {
	eng := engine.New("dev_process")
	req := &models.ExecutionRequest{
		Language:       "python3",
		Code:           "print('A' * 200000)", // 200KB output
		MaxOutputBytes: 1024,                 // 1KB cap
		TimeLimitMs:    2000,
		MemoryLimitMB:  128,
		SandboxType:    "dev_process",
	}

	res, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verdict != models.VerdictOutputLimitExceeded {
		t.Fatalf("expected verdict OUTPUT_LIMIT_EXCEEDED, got %s", res.Verdict)
	}

	if len(res.Stdout) > 1024 {
		t.Fatalf("stdout exceeded buffer cap: %d bytes", len(res.Stdout))
	}
}

func TestPayloadFilesExist(t *testing.T) {
	payloadPaths := []string{
		"../../testdata/payloads/accepted/solution.py",
		"../../testdata/payloads/accepted/solution.cpp",
		"../../testdata/payloads/infinite_loop/loop.py",
		"../../testdata/payloads/infinite_loop/loop.cpp",
		"../../testdata/payloads/memory_hog/oom.py",
		"../../testdata/payloads/memory_hog/oom.cpp",
		"../../testdata/payloads/fork_bomb/bomb.py",
		"../../testdata/payloads/fork_bomb/bomb.cpp",
		"../../testdata/payloads/runtime_error/error.py",
		"../../testdata/payloads/runtime_error/error.cpp",
		"../../testdata/payloads/compile_error/bad.cpp",
	}

	for _, path := range payloadPaths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("payload file does not exist: %s", path)
		}
	}
}
