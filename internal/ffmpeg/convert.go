// Package ffmpeg converts audio files between formats using the ffmpeg CLI.
package ffmpeg

import (
	"fmt"
	"os/exec"
)

// ToOGG converts a WAV file to OGG/OPUS format using ffmpeg.
// Returns an error if ffmpeg is not found on PATH.
func ToOGG(wavPath, oggPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on PATH (required for telegram_voice): %w", err)
	}
	// Telegram requires OGG format with Opus codec for voice messages.
	// -y overwrites the output file without prompting.
	cmd := exec.Command("ffmpeg", "-i", wavPath, "-c:a", "libopus", oggPath, "-y")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg convert: %w\n%s", err, out)
	}
	return nil
}
