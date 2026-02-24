//go:build darwin

package speech

import (
	"fmt"
	"os"
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

// SayToFile renders TTS to a WAV file at the given path.
// macOS `say` outputs AIFF natively, so we write to a temp AIFF
// then convert to WAV with afconvert.
func SayToFile(text string, volume int, path string) error {
	vol := fmt.Sprintf("%.2f", float64(volume)/100.0)
	aiff := path + ".aiff"
	cmd := exec.Command("say", "--volume", vol, "-o", aiff, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("speech to file failed: %w\n%s", err, out)
	}
	defer func() { _ = os.Remove(aiff) }()

	cmd = exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", aiff, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("aiff to wav conversion failed: %w\n%s", err, out)
	}
	return nil
}
