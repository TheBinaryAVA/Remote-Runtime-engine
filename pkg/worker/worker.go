package worker

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/metrics"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/sandbox"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/security"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
)

// ProcessJob executes a single submission job with full isolation, event broadcasting, and state updates.
func ProcessJob(ctx context.Context, job *queue.SubmissionJob, eventBus events.EventBus, stateStore store.StateStore) (retErr error) {
	if job == nil {
		return nil
	}

	submissionID := job.SubmissionID
	totalTestCases := len(job.TestCases)
	if totalTestCases == 0 {
		// If no explicit test cases provided, treat as a single default execution
		job.TestCases = []queue.TestCase{{ID: "tc-default", Input: "", ExpectedOutput: ""}}
		totalTestCases = 1
	}

	// Panic recovery to protect the worker daemon from crashing
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			errDetails := fmt.Sprintf("panic recovered in worker: %v\n%s", r, stack)
			log.Printf("[Worker Panic] submission %s: %s", submissionID, errDetails)

			failedState := &store.SubmissionState{
				SubmissionID:   submissionID,
				Status:         events.StatusFailed,
				Verdict:        models.VerdictSystemError,
				Language:       job.Language,
				TotalTestCases: totalTestCases,
				ErrorDetails:   errDetails,
				UpdatedAt:      time.Now(),
			}
			_ = stateStore.SaveState(ctx, failedState)
			_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
				SubmissionID: submissionID,
				Status:       events.StatusFailed,
				Verdict:      models.VerdictSystemError,
				ErrorDetails: errDetails,
				Timestamp:    time.Now(),
			})
			retErr = fmt.Errorf("worker panic: %v", r)
		}
	}()

	// 1. Resolve Language Handler
	lang, err := languages.Get(job.Language)
	if err != nil {
		return failSubmission(ctx, submissionID, job.Language, totalTestCases, models.VerdictSystemError, err.Error(), nil, eventBus, stateStore)
	}

	// 2. Select Isolation Sandbox
	sb, err := sandbox.SelectSandbox(job.SandboxType)
	if err != nil {
		return failSubmission(ctx, submissionID, job.Language, totalTestCases, models.VerdictSystemError, err.Error(), nil, eventBus, stateStore)
	}

	// 3. Prepare Sandbox Workspace
	execReq := &models.ExecutionRequest{
		ID:             submissionID,
		Language:       job.Language,
		Code:           job.Code,
		TimeLimitMs:    job.TimeLimitMs,
		MemoryLimitMB:  job.MemoryLimitMB,
		CpuQuota:       job.CpuQuota,
		PidsLimit:      job.PidsLimit,
		MaxOutputBytes: job.MaxOutputBytes,
		SandboxType:    job.SandboxType,
	}
	execReq.Normalize()

	sCtx, err := sb.Prepare(ctx, execReq, lang)
	if err != nil {
		return failSubmission(ctx, submissionID, job.Language, totalTestCases, models.VerdictSystemError, fmt.Sprintf("sandbox prepare failed: %v", err), nil, eventBus, stateStore)
	}
	defer func() {
		_ = sb.Cleanup(sCtx)
	}()

	// 4. Compilation Phase (if compiled language like C++)
	var compResult *models.CompilationResult
	if lang.NeedsCompilation() {
		_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
			SubmissionID: submissionID,
			Status:       events.StatusCompiling,
			Timestamp:    time.Now(),
		})

		compResult, err = sb.Compile(ctx, sCtx)
		if err != nil || (compResult != nil && !compResult.Success) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			} else if compResult != nil {
				errStr = compResult.Stderr
			}
			return failSubmission(ctx, submissionID, job.Language, totalTestCases, models.VerdictCompilationError, errStr, compResult, eventBus, stateStore)
		}
	}

	// 5. Testcase Evaluation Phase
	_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
		SubmissionID:   submissionID,
		Status:         events.StatusRunning,
		TotalTestCases: totalTestCases,
		Compilation:    compResult,
		Timestamp:      time.Now(),
	})

	currentState := &store.SubmissionState{
		SubmissionID:   submissionID,
		Status:         events.StatusRunning,
		Verdict:        models.VerdictAccepted,
		Language:       job.Language,
		TotalTestCases: totalTestCases,
		Compilation:    compResult,
		Results:        make([]events.TestCaseResult, 0, totalTestCases),
		CreatedAt:      job.EnqueuedAt,
		UpdatedAt:      time.Now(),
	}
	_ = stateStore.SaveState(ctx, currentState)

	overallVerdict := models.VerdictAccepted
	var maxPeakMemoryMB float64
	var totalWallTimeMs float64
	var totalCpuTimeMs float64
	passedTests := 0

	for i, tc := range job.TestCases {
		tcIndex := i + 1

		// Emit TESTCASE_START
		_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
			SubmissionID:    submissionID,
			Status:          events.StatusTestCaseStart,
			CurrentTestCase: tcIndex,
			TotalTestCases:  totalTestCases,
			Timestamp:       time.Now(),
		})

		// Configure execution request for this testcase
		sCtx.Request.Stdin = tc.Input
		sCtx.Request.ExpectedOutput = tc.ExpectedOutput

		// Execute against warm compiled binary or script
		tcRes, execErr := sb.Execute(ctx, sCtx)
		if execErr != nil {
			tcRes = &models.ExecutionResult{
				ID:             submissionID,
				Verdict:        models.VerdictSystemError,
				ErrorDetails:   execErr.Error(),
				SandboxBackend: sb.Name(),
				ExecutedAt:     time.Now(),
			}
		}

		// Record metrics
		if tcRes.PeakMemoryMB > maxPeakMemoryMB {
			maxPeakMemoryMB = tcRes.PeakMemoryMB
		}
		totalWallTimeMs += tcRes.WallTimeMs
		totalCpuTimeMs += tcRes.CpuTimeMs

		// Hide stdout/stderr if testcase is marked hidden
		stdout := tcRes.Stdout
		stderr := tcRes.Stderr
		if tc.IsHidden && tcRes.Verdict != models.VerdictAccepted {
			stdout = "[Hidden Testcase]"
			stderr = "[Hidden Testcase]"
		}

		resultItem := events.TestCaseResult{
			TestCaseID:   tc.ID,
			Index:        tcIndex,
			Verdict:      tcRes.Verdict,
			ExitCode:     tcRes.ExitCode,
			Stdout:       stdout,
			Stderr:       stderr,
			WallTimeMs:   tcRes.WallTimeMs,
			CpuTimeMs:    tcRes.CpuTimeMs,
			PeakMemoryMB: tcRes.PeakMemoryMB,
			OOMKilled:    tcRes.OOMKilled,
		}
		currentState.Results = append(currentState.Results, resultItem)

		if tcRes.Verdict == models.VerdictAccepted {
			passedTests++
			_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
				SubmissionID:    submissionID,
				Status:          events.StatusTestCasePassed,
				CurrentTestCase: tcIndex,
				TotalTestCases:  totalTestCases,
				TestCaseResult:  &resultItem,
				WallTimeMs:      tcRes.WallTimeMs,
				PeakMemoryMB:    tcRes.PeakMemoryMB,
				Timestamp:       time.Now(),
			})
		} else {
			if overallVerdict == models.VerdictAccepted {
				overallVerdict = tcRes.Verdict
			}
			_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
				SubmissionID:    submissionID,
				Status:          events.StatusTestCaseFailed,
				CurrentTestCase: tcIndex,
				TotalTestCases:  totalTestCases,
				Verdict:         tcRes.Verdict,
				TestCaseResult:  &resultItem,
				WallTimeMs:      tcRes.WallTimeMs,
				PeakMemoryMB:    tcRes.PeakMemoryMB,
				Timestamp:       time.Now(),
			})
		}
	}

	// 6. Complete Submission & Record Telemetry
	now := time.Now()
	currentState.Status = events.StatusCompleted
	currentState.Verdict = overallVerdict
	currentState.PassedTestCases = passedTests
	currentState.PeakMemoryMB = maxPeakMemoryMB
	currentState.TotalWallTimeMs = totalWallTimeMs
	currentState.TotalCpuTimeMs = totalCpuTimeMs
	currentState.CompletedAt = &now
	_ = stateStore.SaveState(ctx, currentState)

	// Record Prometheus Metrics
	metrics.RecordSubmissionMetrics(job.Language, string(overallVerdict), sb.Name(), totalWallTimeMs, totalCpuTimeMs, maxPeakMemoryMB)

	// Security audit for critical violations
	if overallVerdict == models.VerdictMemoryLimitExceeded {
		security.LogViolation(submissionID, job.Language, sb.Name(), security.ViolationMemoryOOM, fmt.Sprintf("Memory limit exceeded: %.2fMB > %dMB", maxPeakMemoryMB, job.MemoryLimitMB))
		metrics.RecordViolation(string(security.ViolationMemoryOOM))
	} else if overallVerdict == models.VerdictTimeLimitExceeded {
		security.LogViolation(submissionID, job.Language, sb.Name(), security.ViolationTimeoutExceeded, fmt.Sprintf("Time limit exceeded: %.2fms > %dms", totalWallTimeMs, job.TimeLimitMs))
		metrics.RecordViolation(string(security.ViolationTimeoutExceeded))
	}

	_ = eventBus.Publish(ctx, submissionID, &events.ExecutionEvent{
		SubmissionID:    submissionID,
		Status:          events.StatusCompleted,
		Verdict:         overallVerdict,
		CurrentTestCase: totalTestCases,
		TotalTestCases:  totalTestCases,
		WallTimeMs:      totalWallTimeMs,
		CpuTimeMs:       totalCpuTimeMs,
		PeakMemoryMB:    maxPeakMemoryMB,
		Compilation:     compResult,
		Timestamp:       now,
	})

	return nil
}

func failSubmission(ctx context.Context, id, lang string, total int, verdict models.Verdict, errDetails string, comp *models.CompilationResult, bus events.EventBus, st store.StateStore) error {
	now := time.Now()
	state := &store.SubmissionState{
		SubmissionID:   id,
		Status:         events.StatusFailed,
		Verdict:        verdict,
		Language:       lang,
		TotalTestCases: total,
		Compilation:    comp,
		ErrorDetails:   errDetails,
		CreatedAt:      now,
		UpdatedAt:      now,
		CompletedAt:    &now,
	}
	_ = st.SaveState(ctx, state)

	metrics.RecordSubmissionMetrics(lang, string(verdict), "unknown", 0, 0, 0)
	if verdict == models.VerdictCompilationError {
		metrics.RecordViolation("COMPILATION_FAILURE")
	} else {
		metrics.RecordViolation(string(security.ViolationSyscallBlocked))
	}

	_ = bus.Publish(ctx, id, &events.ExecutionEvent{
		SubmissionID: id,
		Status:       events.StatusFailed,
		Verdict:      verdict,
		Compilation:  comp,
		ErrorDetails: errDetails,
		Timestamp:    now,
	})
	return nil
}
