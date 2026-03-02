package desktop

import (
	"runtime"
	"testing"
)

func TestAvailable(t *testing.T) {
	got := Available()
	if runtime.GOOS != "windows" && got {
		t.Error("Available() should return false on non-Windows")
	}
}

func TestSwitchToNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	err := SwitchTo(1)
	if err == nil {
		t.Error("SwitchTo should return error on non-Windows")
	}
}

func TestCurrentNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	_, err := Current()
	if err == nil {
		t.Error("Current should return error on non-Windows")
	}
}

func TestCountNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	_, err := Count()
	if err == nil {
		t.Error("Count should return error on non-Windows")
	}
}

func TestHideConsole(t *testing.T) {
	// HideConsole should not panic on any platform.
	HideConsole()
}

func TestRegisterProtocolNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	err := RegisterProtocol("/usr/bin/notify")
	if err == nil {
		t.Error("RegisterProtocol should return error on non-Windows")
	}
}

func TestUnregisterProtocolNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	err := UnregisterProtocol()
	if err == nil {
		t.Error("UnregisterProtocol should return error on non-Windows")
	}
}

func TestIsProtocolRegisteredNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping non-Windows test on Windows")
	}
	if IsProtocolRegistered() {
		t.Error("IsProtocolRegistered should return false on non-Windows")
	}
}
