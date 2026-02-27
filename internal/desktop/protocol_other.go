//go:build !windows

package desktop

import "fmt"

// RegisterProtocol registers the notify:// URI protocol handler.
// Not supported on non-Windows platforms.
func RegisterProtocol(exePath string) error {
	return fmt.Errorf("protocol registration is only supported on Windows")
}

// UnregisterProtocol removes the notify:// URI protocol handler.
// Not supported on non-Windows platforms.
func UnregisterProtocol() error {
	return fmt.Errorf("protocol registration is only supported on Windows")
}

// IsProtocolRegistered checks whether the notify:// protocol handler is registered.
// Always returns false on non-Windows platforms.
func IsProtocolRegistered() bool { return false }
