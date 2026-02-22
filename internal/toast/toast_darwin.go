//go:build darwin

package toast

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

// Show displays a macOS notification using osascript.
func Show(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q`,
		shell.EscapeAppleScript(message), shell.EscapeAppleScript(title))
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}
