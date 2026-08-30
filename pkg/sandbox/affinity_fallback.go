//go:build !linux

package sandbox

// PinProcessToCore is a no-op on non-Linux platforms.
func PinProcessToCore(pid int, coreID int) error {
	return nil
}
