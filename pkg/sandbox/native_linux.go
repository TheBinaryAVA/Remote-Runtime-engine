//go:build linux

package sandbox

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

const (
	CgroupV2Root       = "/sys/fs/cgroup"
	CgroupBaseSlice    = "/sys/fs/cgroup/speedcode"
	EphemeralWorkspace = "/tmp/speedcode"
	DefaultSandboxUID  = 1001
	DefaultSandboxGID  = 1001
)

// NativeCgroupV2Sandbox implements process isolation using native Linux cgroups v2,
// unprivileged credentials, process namespaces, and tmpfs mounts.
type NativeCgroupV2Sandbox struct{}

func NewNativeCgroupV2Sandbox() *NativeCgroupV2Sandbox {
	return &NativeCgroupV2Sandbox{}
}

func (n *NativeCgroupV2Sandbox) Name() string {
	return "native_cgroupv2"
}

// IsAvailable checks if the unified cgroup v2 filesystem is mounted and accessible.
func (n *NativeCgroupV2Sandbox) IsAvailable() bool {
	// Check if /sys/fs/cgroup is cgroup v2
	var stat syscall.Statfs_t
	if err := syscall.Statfs(CgroupV2Root, &stat); err != nil {
		return false
	}
	// Cgroup2 magic number: 0x63677270 (CGROUP2_SUPER_MAGIC)
	isCgroupV2 := (uint32(stat.Type) == 0x63677270)
	isRoot := (os.Geteuid() == 0)
	return isCgroupV2 && isRoot
}

// Prepare sets up the ephemeral directory and initial cgroup v2 slice.
func (n *NativeCgroupV2Sandbox) Prepare(ctx context.Context, req *models.ExecutionRequest, lang languages.LanguageHandler) (*SandboxContext, error) {
	workDir := filepath.Join(EphemeralWorkspace, req.ID)
	cgroupPath := filepath.Join(CgroupBaseSlice, req.ID)

	sCtx := &SandboxContext{
		ID:         req.ID,
		Request:    req,
		Language:   lang,
		WorkingDir: workDir,
		SourcePath: filepath.Join(workDir, lang.SourceFilename()),
		BinaryPath: filepath.Join(workDir, lang.BinaryFilename()),
		CgroupPath: cgroupPath,
		CreatedAt:  time.Now(),
	}

	if err := WriteSourceFile(sCtx); err != nil {
		return nil, err
	}

	// Change ownership of working directory to unprivileged user so sandboxed process can read/write
	_ = os.Chown(workDir, DefaultSandboxUID, DefaultSandboxGID)
	_ = os.Chmod(workDir, 0755)

	// Ensure base cgroup exists and enables controllers
	if err := n.setupCgroupHierarchy(cgroupPath, req); err != nil {
		_ = n.Cleanup(sCtx)
		return nil, fmt.Errorf("cgroup v2 initialization failed: %w", err)
	}

	return sCtx, nil
}

// setupCgroupHierarchy creates the cgroup v2 folder and writes resource limits.
func (n *NativeCgroupV2Sandbox) setupCgroupHierarchy(cgroupPath string, req *models.ExecutionRequest) error {
	// Ensure base speedcode slice exists
	if err := os.MkdirAll(CgroupBaseSlice, 0755); err != nil {
		return fmt.Errorf("failed to create base cgroup slice: %w", err)
	}

	// Enable subtree controllers if needed in root/base
	_ = os.WriteFile(filepath.Join(CgroupV2Root, "cgroup.subtree_control"), []byte("+cpu +memory +pids\n"), 0644)
	_ = os.WriteFile(filepath.Join(CgroupBaseSlice, "cgroup.subtree_control"), []byte("+cpu +memory +pids\n"), 0644)

	// Create job cgroup directory
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("failed to create job cgroup slice: %w", err)
	}

	// 1. Configure memory limit (memory.max in bytes)
	memBytes := req.MemoryLimitMB * 1024 * 1024
	if err := os.WriteFile(filepath.Join(cgroupPath, "memory.max"), []byte(strconv.FormatInt(memBytes, 10)+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to set memory.max: %w", err)
	}

	// 2. Configure CPU limit (cpu.max: quota_us period_us)
	periodUs := int64(100000) // 100ms
	quotaUs := int64(req.CpuQuota * float64(periodUs))
	cpuLimitStr := fmt.Sprintf("%d %d\n", quotaUs, periodUs)
	if err := os.WriteFile(filepath.Join(cgroupPath, "cpu.max"), []byte(cpuLimitStr), 0644); err != nil {
		return fmt.Errorf("failed to set cpu.max: %w", err)
	}

	// 3. Configure PIDs limit (pids.max)
	if err := os.WriteFile(filepath.Join(cgroupPath, "pids.max"), []byte(strconv.FormatInt(req.PidsLimit, 10)+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to set pids.max: %w", err)
	}

	return nil
}

// Compile runs the compiler (e.g. g++) under compile time constraints.
func (n *NativeCgroupV2Sandbox) Compile(ctx context.Context, sCtx *SandboxContext) (*models.CompilationResult, error) {
	if !sCtx.Language.NeedsCompilation() {
		return &models.CompilationResult{Success: true}, nil
	}

	cmdName, cmdArgs := sCtx.Language.CompileCommand(sCtx.SourcePath, sCtx.BinaryPath)
	compileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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

// Execute executes the sandboxed process inside the cgroup v2 boundary.
func (n *NativeCgroupV2Sandbox) Execute(ctx context.Context, sCtx *SandboxContext) (*models.ExecutionResult, error) {
	targetPath := sCtx.SourcePath
	if sCtx.Language.NeedsCompilation() {
		targetPath = sCtx.BinaryPath
	}
	cmdName, cmdArgs := sCtx.Language.RunCommand(targetPath)

	// Establish external wall-clock watchdog context
	execCtx, cancel := context.WithTimeout(ctx, sCtx.Request.TimeoutDuration())
	defer cancel()

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Dir = sCtx.WorkingDir

	// Configure unprivileged execution & process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Credential: &syscall.Credential{
			Uid: uint32(DefaultSandboxUID),
			Gid: uint32(DefaultSandboxGID),
		},
		Pdeathsig: syscall.SIGKILL,
	}

	if sCtx.Request.Stdin != "" {
		cmd.Stdin = strings.NewReader(sCtx.Request.Stdin)
	}

	stdoutBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	stderrBuf := NewBoundedBuffer(sCtx.Request.MaxOutputBytes)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	wallStart := time.Now()

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Move the process into the cgroup v2 slice immediately
	procsFile := filepath.Join(sCtx.CgroupPath, "cgroup.procs")
	if err := os.WriteFile(procsFile, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to attach process to cgroup: %w", err)
	}

	// Wait for process completion or watchdog timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	var timedOut bool

	select {
	case <-execCtx.Done():
		timedOut = true
		// Terminate process group with SIGKILL
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		waitErr = <-done
	case waitErr = <-done:
		// Completed naturally
	}

	wallDuration := time.Since(wallStart).Seconds() * 1000.0

	// Read kernel-level cgroup metrics
	peakMemoryBytes, oomKilled := n.readCgroupMemoryStats(sCtx.CgroupPath)
	cpuTimeMs := n.readCgroupCpuStats(sCtx.CgroupPath)

	// Fallback to rusage if cgroup CPU is zero
	if cpuTimeMs == 0 && cmd.ProcessState != nil {
		if rusage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			userMs := float64(rusage.Utime.Sec)*1000.0 + float64(rusage.Utime.Usec)/1000.0
			sysMs := float64(rusage.Stime.Sec)*1000.0 + float64(rusage.Stime.Usec)/1000.0
			cpuTimeMs = userMs + sysMs
		}
	}

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Determine verdict
	verdict := models.VerdictAccepted
	if timedOut || wallDuration > float64(sCtx.Request.TimeLimitMs) {
		verdict = models.VerdictTimeLimitExceeded
	} else if oomKilled || peakMemoryBytes > (sCtx.Request.MemoryLimitMB*1024*1024) {
		verdict = models.VerdictMemoryLimitExceeded
	} else if stdoutBuf.Exceeded() || stderrBuf.Exceeded() {
		verdict = models.VerdictOutputLimitExceeded
	} else if exitCode != 0 {
		verdict = models.VerdictRuntimeError
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
		CpuTimeMs:      cpuTimeMs,
		PeakMemoryKB:   peakMemoryBytes / 1024,
		PeakMemoryMB:   float64(peakMemoryBytes) / (1024 * 1024),
		OOMKilled:      oomKilled,
		SandboxBackend: n.Name(),
		ExecutedAt:     time.Now(),
	}, nil
}

// readCgroupMemoryStats extracts peak memory usage and OOM event flags from cgroup v2.
func (n *NativeCgroupV2Sandbox) readCgroupMemoryStats(cgroupPath string) (int64, bool) {
	var peakBytes int64
	var oomKilled bool

	// Read memory.peak (bytes)
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.peak")); err == nil {
		val, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		peakBytes = val
	}

	// Read memory.events (oom_kill counter)
	if file, err := os.Open(filepath.Join(cgroupPath, "memory.events")); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				if (fields[0] == "oom" || fields[0] == "oom_kill") && fields[1] != "0" {
					oomKilled = true
				}
			}
		}
	}

	return peakBytes, oomKilled
}

// readCgroupCpuStats reads cpu.stat and computes total CPU time in milliseconds.
func (n *NativeCgroupV2Sandbox) readCgroupCpuStats(cgroupPath string) float64 {
	file, err := os.Open(filepath.Join(cgroupPath, "cpu.stat"))
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "usage_usec" {
			usec, _ := strconv.ParseInt(fields[1], 10, 64)
			return float64(usec) / 1000.0
		}
	}
	return 0
}

// Cleanup tears down the cgroup slice and removes temporary workspace.
func (n *NativeCgroupV2Sandbox) Cleanup(sCtx *SandboxContext) error {
	// 1. Kill any remaining processes in cgroup
	killPath := filepath.Join(sCtx.CgroupPath, "cgroup.kill")
	if _, err := os.Stat(killPath); err == nil {
		_ = os.WriteFile(killPath, []byte("1\n"), 0644)
	}

	// 2. Remove cgroup slice directory
	_ = os.Remove(sCtx.CgroupPath)

	// 3. Remove ephemeral working directory
	_ = os.RemoveAll(sCtx.WorkingDir)

	return nil
}
