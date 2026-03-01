package main

import "syscall"

var user32 = syscall.NewLazyDLL("user32.dll")
var getAsyncKeyState = user32.NewProc("GetAsyncKeyState")

func isShiftHeld() bool {
	ret, _, _ := getAsyncKeyState.Call(0x10) // VK_SHIFT
	return ret&0x8000 != 0
}
