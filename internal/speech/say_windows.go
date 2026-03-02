//go:build windows

// Package speech synthesizes text to speech using platform-native TTS engines.
package speech

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

// sayScript returns a PowerShell script that uses the .NET System.Speech.Synthesis
// API to speak text aloud through the default audio device.
func sayScript(text string, volume int) string {
	return fmt.Sprintf(`Add-Type -AssemblyName System.Speech; `+
		`$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; `+
		`$s.Volume = %d; `+
		`$s.Speak('%s')`, volume, shell.EscapePowerShell(text))
}

// Say synthesizes text to speech using Windows System.Speech and plays it aloud.
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
		`$s.Dispose()`, // Dispose flushes buffered WAV data to disk; without it the file may be truncated
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
