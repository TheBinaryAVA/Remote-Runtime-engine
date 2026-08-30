//go:build linux

package sandbox

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	SYS_SCHED_SETAFFINITY = 203 // x86_64 syscall number
)

// PinProcessToCore binds the specified process PID to a single dedicated CPU core.
func PinProcessToCore(pid int, coreID int) error {
	if coreID < 0 {
		return nil
	}

	// Try setting via sched_setaffinity syscall
	var mask [16]uintptr
	byteIndex := coreID / 64
	bitIndex := uint(coreID % 64)

	if byteIndex < len(mask) {
		mask[byteIndex] |= (1 << bitIndex)
		_, _, errno := syscall.RawSyscall(
			SYS_SCHED_SETAFFINITY,
			uintptr(pid),
			uintptr(unsafe.Sizeof(mask)),
			uintptr(unsafe.Pointer(&mask[0])),
		)
		if errno == 0 {
			return nil
		}
	}

	// Fallback to taskset CLI command if available
	cmd := exec.Command("taskset", "-cp", strconv.Itoa(coreID), strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pin PID %d to core %d: %w", pid, coreID, err)
	}

	return nil
}
