package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/sandbox"
)

// Engine coordinates the end-to-end compilation, sandboxed isolation, metric aggregation, and cleanup.
type Engine struct {
	SandboxBackend string // "auto", "native", "docker"
}

// New creates a new instance of the execution engine.
func New(sandboxBackend string) *Engine {
	return &Engine{
		SandboxBackend: sandboxBackend,
	}
}

// Run processes a single code execution request through the full isolated pipeline.
func (e *Engine) Run(ctx context.Context, req *models.ExecutionRequest) (*models.ExecutionResult, error) {
	if req == nil {
		return nil, fmt.Errorf("execution request cannot be nil")
	}
	req.Normalize()

	// 1. Resolve language handler
	lang, err := languages.Get(req.Language)
	if err != nil {
		return &models.ExecutionResult{
			ID:           req.ID,
			Verdict:      models.VerdictSystemError,
			ErrorDetails: err.Error(),
			ExecutedAt:   time.Now(),
		}, nil
	}

	// 2. Select isolation sandbox backend
	sb, err := sandbox.SelectSandbox(req.SandboxType)
	if err != nil && e.SandboxBackend != "" {
		sb, err = sandbox.SelectSandbox(e.SandboxBackend)
	}
	if err != nil {
		return &models.ExecutionResult{
			ID:           req.ID,
			Verdict:      models.VerdictSystemError,
			ErrorDetails: fmt.Sprintf("failed to select sandbox: %v", err),
			ExecutedAt:   time.Now(),
		}, nil
	}

	// 3. Prepare sandbox workspace
	sCtx, err := sb.Prepare(ctx, req, lang)
	if err != nil {
		return &models.ExecutionResult{
			ID:             req.ID,
			Verdict:        models.VerdictSystemError,
			ErrorDetails:   fmt.Sprintf("failed to prepare sandbox: %v", err),
			SandboxBackend: sb.Name(),
			ExecutedAt:     time.Now(),
		}, nil
	}
	defer func() {
		_ = sb.Cleanup(sCtx)
	}()

	// 4. Compilation phase (if required)
	if lang.NeedsCompilation() {
		compResult, err := sb.Compile(ctx, sCtx)
		if err != nil {
			return &models.ExecutionResult{
				ID:             req.ID,
				Verdict:        models.VerdictCompilationError,
				Compilation:    compResult,
				ErrorDetails:   fmt.Sprintf("compilation execution error: %v", err),
				SandboxBackend: sb.Name(),
				ExecutedAt:     time.Now(),
			}, nil
		}

		if !compResult.Success {
			return &models.ExecutionResult{
				ID:             req.ID,
				Verdict:        models.VerdictCompilationError,
				ExitCode:       compResult.ExitCode,
				Stdout:         compResult.Stdout,
				Stderr:         compResult.Stderr,
				Compilation:    compResult,
				SandboxBackend: sb.Name(),
				ExecutedAt:     time.Now(),
			}, nil
		}
	}

	// 5. Sandboxed execution phase
	result, err := sb.Execute(ctx, sCtx)
	if err != nil {
		return &models.ExecutionResult{
			ID:             req.ID,
			Verdict:        models.VerdictSystemError,
			ErrorDetails:   fmt.Sprintf("execution failed: %v", err),
			SandboxBackend: sb.Name(),
			ExecutedAt:     time.Now(),
		}, nil
	}

	return result, nil
}
