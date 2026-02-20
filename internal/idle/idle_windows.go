package idle

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	pGetLastInputInfo  = user32.NewProc("GetLastInputInfo")
	pGetTickCount64    = kernel32.NewProc("GetTickCount64")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// IdleSeconds returns the number of seconds since the last keyboard or
// mouse input on Windows, using GetLastInputInfo and GetTickCount64.
func IdleSeconds() (float64, error) {
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))

	r, _, err := pGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r == 0 {
		return 0, fmt.Errorf("GetLastInputInfo: %w", err)
	}

	r2, _, _ := pGetTickCount64.Call()
	tickMs := uint64(r2)

	idleMs := tickMs - uint64(lii.dwTime)
	return float64(idleMs) / 1000.0, nil
}
