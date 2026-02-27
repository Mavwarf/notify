//go:build linux

package toast

import (
	"fmt"
	"os/exec"
)

// Show displays a Linux desktop notification using notify-send.
// The desktop parameter is ignored on Linux.
func Show(title, message string, desktop *int) error {
	cmd := exec.Command("notify-send", title, message)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}
