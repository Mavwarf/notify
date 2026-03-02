//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var findWindowW = user32.NewProc("FindWindowW")
var setWindowPos = user32.NewProc("SetWindowPos")

const (
	hwndTopmost   = ^uintptr(0) // -1
	hwndNotopmost = ^uintptr(1) // -2
	swpNomove     = 0x0002
	swpNosize     = 0x0001
	swpNoactivate = 0x0010
)

func setAlwaysOnTop(on bool) {
	title, _ := syscall.UTF16PtrFromString("notify dashboard")
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
