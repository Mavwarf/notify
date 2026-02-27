//go:build !windows

package desktop

import "fmt"

// Available reports whether virtual desktop switching is supported.
// Always returns false on non-Windows platforms.
func Available() bool { return false }

// SwitchTo switches to the given virtual desktop (1-indexed).
// Not supported on non-Windows platforms.
func SwitchTo(n int) error {
	return fmt.Errorf("virtual desktop switching is only supported on Windows")
}

// Current returns the current virtual desktop number (1-indexed).
// Not supported on non-Windows platforms.
func Current() (int, error) {
	return 0, fmt.Errorf("virtual desktop switching is only supported on Windows")
}

// Count returns the number of virtual desktops.
// Not supported on non-Windows platforms.
func Count() (int, error) {
	return 0, fmt.Errorf("virtual desktop switching is only supported on Windows")
}
