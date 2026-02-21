//go:build linux

package speech

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func Say(text string, volume int) error {
	// espeak amplitude: 0-200, map 0-100 to 0-200
	amp := strconv.Itoa(volume * 2)
	for _, bin := range []string{"espeak-ng", "espeak"} {
		if path, err := exec.LookPath(bin); err == nil {
			cmd := exec.Command(path, "--amplitude", amp, text)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("speech failed: %w\n%s", err, out)
			}
			return nil
		}
	}
	return fmt.Errorf("speech not available: install espeak-ng or espeak")
}

// SayToFile renders TTS to a WAV file at the given path.
// espeak-ng/espeak --stdout outputs WAV to stdout.
func SayToFile(text string, volume int, outPath string) error {
	amp := strconv.Itoa(volume * 2)
	for _, bin := range []string{"espeak-ng", "espeak"} {
		if binPath, err := exec.LookPath(bin); err == nil {
			cmd := exec.Command(binPath, "--amplitude", amp, "--stdout", text)
			data, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("speech to file failed: %w", err)
			}
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return fmt.Errorf("writing wav: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("speech not available: install espeak-ng or espeak")
}
