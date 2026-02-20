//go:build windows

package speech

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

func Say(text string, volume int) error {
	script := fmt.Sprintf(`Add-Type -AssemblyName System.Speech; `+
		`$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; `+
		`$s.Volume = %d; `+
		`$s.Speak('%s')`, volume, shell.EscapePowerShell(text))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("speech failed: %w\n%s", err, out)
	}
	return nil
}
