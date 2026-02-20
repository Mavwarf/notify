//go:build darwin

package toast

import (
	"fmt"
	"os/exec"
	"strings"
)

// Show displays a macOS notification using osascript.
func Show(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q`,
		escapeAppleScript(message), escapeAppleScript(title))
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}

func escapeAppleScript(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
