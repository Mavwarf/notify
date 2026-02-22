package audio

import (
	"testing"
	"time"
)

func TestSoundsRegistryComplete(t *testing.T) {
	expected := []string{"warning", "success", "error", "info", "alert", "notification", "blip"}
	for _, name := range expected {
		if _, ok := Sounds[name]; !ok {
			t.Errorf("missing sound %q in registry", name)
		}
	}
}

func TestGeneratePCMLength(t *testing.T) {
	def := SoundDefinition{
		Segments: []ToneSegment{
			{Frequency: 440, Duration: 100 * time.Millisecond, Volume: 0.5},
		},
	}
	pcm := GeneratePCM(def)
	// 44100 * 0.1s = 4410 samples, 4 bytes each (stereo 16-bit)
	want := 4410 * 4
	if len(pcm) != want {
		t.Errorf("len(pcm) = %d, want %d", len(pcm), want)
	}
}

func TestGeneratePCMSilence(t *testing.T) {
	def := SoundDefinition{
		Segments: []ToneSegment{
			{Frequency: 0, Duration: 50 * time.Millisecond, Volume: 0},
		},
	}
	pcm := GeneratePCM(def)
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(pcm[i]) | int16(pcm[i+1])<<8
		if sample != 0 {
			t.Fatalf("expected silence, got sample %d at offset %d", sample, i)
		}
	}
}

func TestGeneratePCMMultipleSegments(t *testing.T) {
	def := SoundDefinition{
		Segments: []ToneSegment{
			{Frequency: 440, Duration: 50 * time.Millisecond, Volume: 0.5},
			{Frequency: 880, Duration: 50 * time.Millisecond, Volume: 0.5},
		},
	}
	pcm := GeneratePCM(def)
	// Two segments of 50ms each = 100ms total
	want := int(float64(SampleRate)*0.05) * 2 * 4
	if len(pcm) != want {
		t.Errorf("len(pcm) = %d, want %d", len(pcm), want)
	}
}

func TestApplyVolume16Full(t *testing.T) {
	data := []byte{0x00, 0x40} // sample = 16384
	orig := make([]byte, len(data))
	copy(orig, data)
	applyVolume16(data, 1.0)
	if data[0] != orig[0] || data[1] != orig[1] {
		t.Errorf("full volume should not modify data")
	}
}

func TestApplyVolume16Half(t *testing.T) {
	// sample = 16384 (0x4000), half = 8192 (0x2000)
	data := []byte{0x00, 0x40}
	applyVolume16(data, 0.5)
	sample := int16(data[0]) | int16(data[1])<<8
	if sample != 8192 {
		t.Errorf("half volume sample = %d, want 8192", sample)
	}
}

func TestApplyVolume16Zero(t *testing.T) {
	data := []byte{0x00, 0x40}
	applyVolume16(data, 0.0)
	sample := int16(data[0]) | int16(data[1])<<8
	if sample != 0 {
		t.Errorf("zero volume sample = %d, want 0", sample)
	}
}
