package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/worker"
)

func TestWorker_ProcessJob_MultiTestCase(t *testing.T) {
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()
	ctx := context.Background()

	job := &queue.SubmissionJob{
		SubmissionID:  "sub-worker-multi",
		Language:      "python3",
		Code:          "a, b = map(int, input().split())\nprint(a + b)",
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
		TestCases: []queue.TestCase{
			{ID: "tc-1", Input: "2 3\n", ExpectedOutput: "5\n"},
			{ID: "tc-2", Input: "10 20\n", ExpectedOutput: "30\n"},
			{ID: "tc-3", Input: "100 200\n", ExpectedOutput: "300\n"},
		},
	}

	err := worker.ProcessJob(ctx, job, bus, st)
	if err != nil {
		t.Fatalf("unexpected error processing job: %v", err)
	}

	state, err := st.GetState(ctx, "sub-worker-multi")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}

	if state.Status != events.StatusCompleted {
		t.Fatalf("expected StatusCompleted, got %s", state.Status)
	}

	if state.Verdict != models.VerdictAccepted {
		t.Fatalf("expected VerdictAccepted, got %s", state.Verdict)
	}

	if state.PassedTestCases != 3 || len(state.Results) != 3 {
		t.Fatalf("expected 3 passed test cases, got %d passed out of %d results", state.PassedTestCases, len(state.Results))
	}
}

func TestWorkerPool_Lifecycle(t *testing.T) {
	q := queue.NewMemoryQueue(10)
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolCfg := worker.PoolConfig{
		Concurrency: 2,
		PollTimeout: 100 * time.Millisecond,
		WorkerID:    "test-pool",
	}

	pool := worker.NewWorkerPool(poolCfg, q, bus, st)
	pool.Start(ctx)

	// Enqueue a job
	job := &queue.SubmissionJob{
		SubmissionID:  "sub-pool-1",
		Language:      "python3",
		Code:          "print('hello pool')",
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}
	_ = q.Enqueue(ctx, job)

	// Wait for job to process
	time.Sleep(500 * time.Millisecond)

	state, err := st.GetState(ctx, "sub-pool-1")
	if err != nil || state.Status != events.StatusCompleted {
		t.Fatalf("job was not completed by pool: state=%+v, err=%v", state, err)
	}

	pool.Stop()
}
