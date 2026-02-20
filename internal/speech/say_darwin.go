//go:build darwin

package speech

import (
	"fmt"
	"os/exec"
)

func Say(text string, volume int) error {
	// macOS say uses 0.0-1.0 scale
	vol := fmt.Sprintf("%.2f", float64(volume)/100.0)
	cmd := exec.Command("say", "--volume", vol, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("speech failed: %w\n%s", err, out)
	}
	return nil
}
