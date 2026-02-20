//go:build linux

package speech

import (
	"fmt"
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
