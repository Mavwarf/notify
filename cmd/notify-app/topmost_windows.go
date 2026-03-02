//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var findWindowW = user32.NewProc("FindWindowW")
var setWindowPos = user32.NewProc("SetWindowPos")

const (
	hwndTopmost   = ^uintptr(0) // -1: Win32 sentinel — place window above all non-topmost windows
	hwndNotopmost = ^uintptr(1) // -2: Win32 sentinel — remove topmost status, place above non-topmost
	swpNomove     = 0x0002
	swpNosize     = 0x0001
	swpNoactivate = 0x0010
)

// setAlwaysOnTop toggles the window's always-on-top (topmost) state using
// Win32 SetWindowPos. The window is located by title via FindWindowW.
func setAlwaysOnTop(on bool) {
	title, _ := syscall.UTF16PtrFromString("notify dashboard")
	// Pass nil (0) for class name — FindWindowW matches by window title only.
	hwnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return
	}
	insertAfter := hwndNotopmost
	if on {
		insertAfter = hwndTopmost
	}
	setWindowPos.Call(hwnd, insertAfter, 0, 0, 0, 0, swpNomove|swpNosize|swpNoactivate)
}
