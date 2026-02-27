//go:build windows

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var dll *syscall.LazyDLL

func init() {
	// Look for DLL next to the running executable.
	exe, err := os.Executable()
	if err == nil {
		dll = syscall.NewLazyDLL(filepath.Join(filepath.Dir(exe), "VirtualDesktopAccessor.dll"))
	}
}

// Available reports whether the VirtualDesktopAccessor DLL is loadable.
func Available() bool {
	if dll == nil {
		return false
	}
	return dll.Load() == nil
}

// SwitchTo switches to the given virtual desktop (1-indexed).
func SwitchTo(n int) error {
	if dll == nil {
		return fmt.Errorf("VirtualDesktopAccessor.dll not found")
	}
	proc := dll.NewProc("GoToDesktopNumber")
	if err := proc.Find(); err != nil {
		return fmt.Errorf("GoToDesktopNumber not found in DLL: %w", err)
	}
	// DLL is 0-indexed; callers use 1-indexed.
	ret, _, _ := proc.Call(uintptr(n - 1))
	if ret != 0 {
		return fmt.Errorf("GoToDesktopNumber(%d) returned %d", n-1, ret)
	}
	return nil
}

// Current returns the current virtual desktop number (1-indexed).
func Current() (int, error) {
	if dll == nil {
		return 0, fmt.Errorf("VirtualDesktopAccessor.dll not found")
	}
	proc := dll.NewProc("GetCurrentDesktopNumber")
	if err := proc.Find(); err != nil {
		return 0, fmt.Errorf("GetCurrentDesktopNumber not found in DLL: %w", err)
	}
	ret, _, _ := proc.Call()
	return int(ret) + 1, nil // convert 0-indexed to 1-indexed
}

// HideConsole detaches this process from its console window so that
// Windows has no window to refocus when the process exits.
func HideConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("FreeConsole")
	proc.Call()
}

// Count returns the number of virtual desktops.
func Count() (int, error) {
	if dll == nil {
		return 0, fmt.Errorf("VirtualDesktopAccessor.dll not found")
	}
	proc := dll.NewProc("GetDesktopCount")
	if err := proc.Find(); err != nil {
		return 0, fmt.Errorf("GetDesktopCount not found in DLL: %w", err)
	}
	ret, _, _ := proc.Call()
	return int(ret), nil
}
