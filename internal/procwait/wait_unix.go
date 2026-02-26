//go:build linux || darwin

package procwait

import (
	"fmt"
	"syscall"
	"time"
)

// Wait blocks until the process with the given PID exits.
// Polls with syscall.Kill(pid, 0) every 500ms. Signal 0 doesn't send
// a signal — it just checks whether the process exists. Returns an
// error immediately if the process doesn't exist at the start.
func Wait(pid int) error {
	// Check that the process exists before entering the poll loop.
	if err := syscall.Kill(pid, 0); err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	for {
		time.Sleep(500 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err != nil {
			return nil // process exited
		}
	}
}
