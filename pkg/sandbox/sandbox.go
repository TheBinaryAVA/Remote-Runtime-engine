package sandbox

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

// SandboxContext holds state and file paths for an isolated execution lifecycle.
type SandboxContext struct {
	ID         string
	Request    *models.ExecutionRequest
	Language   languages.LanguageHandler
	WorkingDir string
	SourcePath string
	BinaryPath string
	CgroupPath string
	CreatedAt  time.Time
}

// Sandbox defines the isolation backend interface.
type Sandbox interface {
	// Name returns the backend identifier.
	Name() string

	// IsAvailable returns true if the host environment supports this backend.
	IsAvailable() bool

	// Prepare initializes ephemeral directories and writes submission code.
	Prepare(ctx context.Context, req *models.ExecutionRequest, lang languages.LanguageHandler) (*SandboxContext, error)

	// Compile compiles source code if required by the language.
	Compile(ctx context.Context, sCtx *SandboxContext) (*models.CompilationResult, error)

	// Execute runs the program within cgroup v2/container isolation and collects metrics.
	Execute(ctx context.Context, sCtx *SandboxContext) (*models.ExecutionResult, error)

	// Cleanup cleans up ephemeral mounts, cgroups, and temporary disk files.
	Cleanup(sCtx *SandboxContext) error
}

// SelectSandbox selects an appropriate sandbox backend based on preference and host capabilities.
func SelectSandbox(preference string) (Sandbox, error) {
	native := NewNativeCgroupV2Sandbox()
	docker := NewDockerSandbox()
	devSandbox := NewDevProcessSandbox()

	switch preference {
	case "native", "cgroup", "cgroupv2":
		if !native.IsAvailable() {
			return nil, fmt.Errorf("native cgroup v2 sandbox is not available on this host (%s, root required)", runtime.GOOS)
		}
		return native, nil

	case "docker", "container":
		if !docker.IsAvailable() {
			return nil, fmt.Errorf("docker sandbox is not available or Docker daemon is unreachable")
		}
		return docker, nil

	case "dev_process", "dev", "process":
		return devSandbox, nil

	case "auto", "":
		// Preference: Native Linux cgroup v2 if available and running as root on Linux; else Docker; else DevProcess fallback.
		if runtime.GOOS == "linux" && native.IsAvailable() {
			return native, nil
		}
		if docker.IsAvailable() {
			return docker, nil
		}
		return devSandbox, nil

	default:
		return nil, fmt.Errorf("unknown sandbox backend preference: '%s'", preference)
	}
}

// WriteSourceFile helper creates ephemeral workspace and writes the source file safely.
func WriteSourceFile(sCtx *SandboxContext) error {
	if err := os.MkdirAll(sCtx.WorkingDir, 0755); err != nil {
		return fmt.Errorf("failed to create sandbox workspace: %w", err)
	}
	if err := os.WriteFile(sCtx.SourcePath, []byte(sCtx.Request.Code), 0644); err != nil {
		return fmt.Errorf("failed to write source code: %w", err)
	}
	return nil
}
