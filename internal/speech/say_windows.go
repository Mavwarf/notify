//go:build windows

package speech

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

// sayScript returns the PowerShell script for speaking text aloud.
func sayScript(text string, volume int) string {
	return fmt.Sprintf(`Add-Type -AssemblyName System.Speech; `+
		`$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; `+
		`$s.Volume = %d; `+
		`$s.Speak('%s')`, volume, shell.EscapePowerShell(text))
}

func Say(text string, volume int) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", sayScript(text, volume))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("speech failed: %w\n%s", err, out)
	}
	return nil
}

// sayToFileScript returns the PowerShell script for rendering TTS to a WAV file.
func sayToFileScript(text string, volume int, path string) string {
	return fmt.Sprintf(`Add-Type -AssemblyName System.Speech; `+
		`$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; `+
		`$s.Volume = %d; `+
		`$s.SetOutputToWaveFile('%s'); `+
		`$s.Speak('%s'); `+
		`$s.Dispose()`,
		volume, shell.EscapePowerShell(path), shell.EscapePowerShell(text))
}

// SayToFile renders TTS to a WAV file at the given path.
func SayToFile(text string, volume int, path string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", sayToFileScript(text, volume, path))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("speech to file failed: %w\n%s", err, out)
	}
	return nil
}
