package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

const (
	DefaultRunnerImage = "speedcode-runner:latest"
	FallbackRunnerImage = "ubuntu:22.04"
)

// DockerSandbox executes code inside ephemeral, constrained OCI/Docker containers.
type DockerSandbox struct {
	ImageName string
}

func NewDockerSandbox() *DockerSandbox {
	image := os.Getenv("RUNNER_IMAGE")
	if image == "" {
		image = FallbackRunnerImage
	}
	return &DockerSandbox{
		ImageName: image,
	}
}

func (d *DockerSandbox) Name() string {
	return "docker"
}

// IsAvailable checks if the Docker CLI is installed and can communicate with the daemon.
func (d *DockerSandbox) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// Prepare creates an isolated temporary directory and writes the source file.
func (d *DockerSandbox) Prepare(ctx context.Context, req *models.ExecutionRequest, lang languages.LanguageHandler) (*SandboxContext, error) {
	tempDir, err := os.MkdirTemp("", "speedcode-docker-"+req.ID+"-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create docker workspace: %w", err)
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

// Compile compiles the source file inside a compilation container if needed.
func (d *DockerSandbox) Compile(ctx context.Context, sCtx *SandboxContext) (*models.CompilationResult, error) {
	if !sCtx.Language.NeedsCompilation() {
		return &models.CompilationResult{Success: true}, nil
	}

	compileCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmdName, cmdArgs := sCtx.Language.CompileCommand(sCtx.Language.SourceFilename(), sCtx.Language.BinaryFilename())
	fullCommand := append([]string{cmdName}, cmdArgs...)

	containerArgs := []string{
		"run",
		"--rm",
		"--network", "none",
		"-v", fmt.Sprintf("%s:/workspace:rw", sCtx.WorkingDir),
		"-w", "/workspace",
		d.ImageName,
	}
	containerArgs = append(containerArgs, fullCommand...)

	cmd := exec.CommandContext(compileCtx, "docker", containerArgs...)

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

// Execute runs the program inside an ephemeral, resource-constrained container.
func (d *DockerSandbox) Execute(ctx context.Context, sCtx *SandboxContext) (*models.ExecutionResult, error) {
	containerName := fmt.Sprintf("speedcode-exec-%s", sCtx.ID)

	targetFile := filepath.Join("/workspace", sCtx.Language.SourceFilename())
	if sCtx.Language.NeedsCompilation() {
		targetFile = filepath.Join("/workspace", sCtx.Language.BinaryFilename())
	}
	cmdName, cmdArgs := sCtx.Language.RunCommand(targetFile)
	execCommand := append([]string{cmdName}, cmdArgs...)

	pinnedCore := AllocateNextCore()

	// Configure strict container isolation, cgroup limits, and security boundaries
	dockerArgs := []string{
		"run",
		"-i",
		"--name", containerName,
		"--network", "none",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=64m",
		"--memory", fmt.Sprintf("%dm", sCtx.Request.MemoryLimitMB),
		"--memory-swap", fmt.Sprintf("%dm", sCtx.Request.MemoryLimitMB),
		"--cpus", fmt.Sprintf("%.2f", sCtx.Request.CpuQuota),
		"--cpuset-cpus", fmt.Sprintf("%d", pinnedCore),
		"--pids-limit", fmt.Sprintf("%d", sCtx.Request.PidsLimit),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"-v", fmt.Sprintf("%s:/workspace:ro", sCtx.WorkingDir),
		"-w", "/workspace",
		d.ImageName,
	}
	dockerArgs = append(dockerArgs, execCommand...)

	// Wall-clock watchdog
	execCtx, cancel := context.WithTimeout(ctx, sCtx.Request.TimeoutDuration()+500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", dockerArgs...)

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

	// Inspect container for exit code and resource consumption before removal
	exitCode, oomKilled, peakMemoryBytes := d.inspectContainer(containerName)

	// Ensure container removal
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	if err != nil && exitCode == 0 {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Determine verdict
	verdict := models.VerdictAccepted
	if wallDuration > float64(sCtx.Request.TimeLimitMs) || exitCode == 124 || exitCode == 137 && !oomKilled {
		if wallDuration >= float64(sCtx.Request.TimeLimitMs) {
			verdict = models.VerdictTimeLimitExceeded
		}
	}
	if oomKilled || exitCode == 137 && oomKilled {
		verdict = models.VerdictMemoryLimitExceeded
	} else if verdict == models.VerdictAccepted && exitCode != 0 {
		verdict = models.VerdictRuntimeError
	} else if stdoutBuf.Exceeded() || stderrBuf.Exceeded() {
		verdict = models.VerdictOutputLimitExceeded
	} else if verdict == models.VerdictAccepted && sCtx.Request.ExpectedOutput != "" {
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
		CpuTimeMs:      wallDuration * 0.9, // Estimated CPU time
		PeakMemoryKB:   peakMemoryBytes / 1024,
		PeakMemoryMB:   float64(peakMemoryBytes) / (1024 * 1024),
		OOMKilled:      oomKilled,
		SandboxBackend: d.Name(),
		ExecutedAt:     time.Now(),
	}, nil
}

type containerInspectState struct {
	State struct {
		ExitCode  int  `json:"ExitCode"`
		OOMKilled bool `json:"OOMKilled"`
	} `json:"State"`
}

func (d *DockerSandbox) inspectContainer(containerName string) (exitCode int, oomKilled bool, peakMemory int64) {
	out, err := exec.Command("docker", "inspect", containerName).Output()
	if err != nil {
		return 0, false, 0
	}

	var inspects []containerInspectState
	if err := json.Unmarshal(out, &inspects); err == nil && len(inspects) > 0 {
		return inspects[0].State.ExitCode, inspects[0].State.OOMKilled, 0
	}

	return 0, false, 0
}

func (d *DockerSandbox) Cleanup(sCtx *SandboxContext) error {
	return os.RemoveAll(sCtx.WorkingDir)
}
