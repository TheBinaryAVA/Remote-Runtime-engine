package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

// DevProcessSandbox is a lightweight cross-platform runner with watchdog timers
// used for local developer iteration and unit tests where root cgroups are unavailable.
type DevProcessSandbox struct{}

func NewDevProcessSandbox() *DevProcessSandbox {
	return &DevProcessSandbox{}
}

func (d *DevProcessSandbox) Name() string {
	return "dev_process"
}

func (d *DevProcessSandbox) IsAvailable() bool {
	return true
}

func (d *DevProcessSandbox) Prepare(ctx context.Context, req *models.ExecutionRequest, lang languages.LanguageHandler) (*SandboxContext, error) {
	tempDir, err := os.MkdirTemp("", "speedcode-dev-"+req.ID+"-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create dev workspace: %w", err)
	}

	sCtx := &SandboxContext{
		ID:         req.ID,
		Request:    req,
		Language:   lang,
		WorkingDir: tempDir,
		SourcePath: filepath.Join(tempDir, lang.SourceFilename()),
		BinaryPath: filepath.Join(tempDir, lang.BinaryFilename()),
		CreatedAt:  time.Now(),
	}

	if err := WriteSourceFile(sCtx); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	return sCtx, nil
}

func (d *DevProcessSandbox) Compile(ctx context.Context, sCtx *SandboxContext) (*models.CompilationResult, error) {
	if !sCtx.Language.NeedsCompilation() {
		return &models.CompilationResult{Success: true}, nil
	}

	cmdName, cmdArgs := sCtx.Language.CompileCommand(sCtx.SourcePath, sCtx.BinaryPath)
	compileCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(compileCtx, cmdName, cmdArgs...)
	cmd.Dir = sCtx.WorkingDir

	stdoutBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	stderrBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Seconds() * 1000.0

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &models.CompilationResult{
		Success:  exitCode == 0,
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		TimeMs:   duration,
	}, nil
}

func (d *DevProcessSandbox) Execute(ctx context.Context, sCtx *SandboxContext) (*models.ExecutionResult, error) {
	targetPath := sCtx.SourcePath
	if sCtx.Language.NeedsCompilation() {
		targetPath = sCtx.BinaryPath
	}
	cmdName, cmdArgs := sCtx.Language.RunCommand(targetPath)

	execCtx, cancel := context.WithTimeout(ctx, sCtx.Request.TimeoutDuration())
	defer cancel()

	cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
	cmd.Dir = sCtx.WorkingDir

	if sCtx.Request.Stdin != "" {
		cmd.Stdin = strings.NewReader(sCtx.Request.Stdin)
	}

	stdoutBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	stderrBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	wallStart := time.Now()
	err := cmd.Run()
	wallDuration := time.Since(wallStart).Seconds() * 1000.0

	timedOut := execCtx.Err() == context.DeadlineExceeded

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	verdict := models.VerdictAccepted
	if timedOut || wallDuration > float64(sCtx.Request.TimeLimitMs) {
		verdict = models.VerdictTimeLimitExceeded
	} else if exitCode != 0 {
		verdict = models.VerdictRuntimeError
	} else if stdoutBuf.Exceeded() || stderrBuf.Exceeded() {
		verdict = models.VerdictOutputLimitExceeded
	} else if sCtx.Request.ExpectedOutput != "" {
		if strings.TrimRight(stdoutBuf.String(), "\r\n") != strings.TrimRight(sCtx.Request.ExpectedOutput, "\r\n") {
			verdict = models.VerdictWrongAnswer
		}
	}

	return &models.ExecutionResult{
		ID:             sCtx.ID,
		Verdict:        verdict,
		ExitCode:       exitCode,
		Stdout:         stdoutBuf.String(),
		Stderr:         stderrBuf.String(),
		WallTimeMs:     wallDuration,
		CpuTimeMs:      wallDuration * 0.9,
		PeakMemoryKB:   1024,
		PeakMemoryMB:   1.0,
		OOMKilled:      false,
		SandboxBackend: d.Name(),
		ExecutedAt:     time.Now(),
	}, nil
}

func (d *DevProcessSandbox) Cleanup(sCtx *SandboxContext) error {
	return os.RemoveAll(sCtx.WorkingDir)
}
