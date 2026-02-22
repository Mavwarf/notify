package ffmpeg

import (
	"os/exec"
	"strings"
	"testing"
)

func TestToOGGMissingFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		t.Skip("ffmpeg is installed, skipping missing-ffmpeg test")
	}

	err := ToOGG("input.wav", "output.ogg")
	if err == nil {
		t.Fatal("expected error when ffmpeg is not installed")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Errorf("error should mention ffmpeg, got: %v", err)
	}
}

func TestToOGGBadInput(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping bad-input test")
	}

	err := ToOGG("/nonexistent/file.wav", t.TempDir()+"/output.ogg")
	if err == nil {
		t.Fatal("expected error for nonexistent input file")
	}
}
