package config

import (
	"testing"

	"github.com/Mavwarf/notify/internal/audio"
)

// TestBuiltinSoundsMatchAudio ensures that builtinSounds in config stays in
// sync with audio.Sounds. If a sound is added to or removed from audio.Sounds,
// this test will fail until builtinSounds is updated to match.
func TestBuiltinSoundsMatchAudio(t *testing.T) {
	for name := range audio.Sounds {
		if !builtinSounds[name] {
			t.Errorf("audio.Sounds has %q but config.builtinSounds does not", name)
		}
	}
	for name := range builtinSounds {
		if _, ok := audio.Sounds[name]; !ok {
			t.Errorf("config.builtinSounds has %q but audio.Sounds does not", name)
		}
	}
}
