package procwait

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Wait blocks until the process with the given PID exits.
// Uses OpenProcess(SYNCHRONIZE) + WaitForSingleObject for efficient
// kernel-level blocking (no polling). Returns an error if the process
// doesn't exist or can't be opened.
func Wait(pid int) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}
	defer windows.CloseHandle(h)

	event, err := windows.WaitForSingleObject(h, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("waiting for process %d: %w", pid, err)
	}
	if event != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("unexpected wait result for process %d: %d", pid, event)
	}
	return nil
}
