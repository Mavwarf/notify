package audio

import (
	"math"
	"time"
)

// ToneSegment defines a single tone burst with frequency, duration, and volume.
type ToneSegment struct {
	Frequency float64
	Duration  time.Duration
	Volume    float64 // 0.0 to 1.0
}

// SoundDefinition describes a named sound composed of one or more tone segments.
type SoundDefinition struct {
	Name        string
	Description string
	Segments    []ToneSegment
}

const SampleRate = 44100

// Sounds is the registry of all available sound effects.
var Sounds = map[string]SoundDefinition{
	"warning": {
		Name:        "warning",
		Description: "Two-tone alternating warning signal",
		Segments: []ToneSegment{
			{Frequency: 800, Duration: 150 * time.Millisecond, Volume: 0.7},
			{Frequency: 600, Duration: 150 * time.Millisecond, Volume: 0.7},
			{Frequency: 800, Duration: 150 * time.Millisecond, Volume: 0.7},
			{Frequency: 600, Duration: 150 * time.Millisecond, Volume: 0.7},
		},
	},
	"success": {
		Name:        "success",
		Description: "Ascending major chord chime",
		Segments: []ToneSegment{
			{Frequency: 523.25, Duration: 120 * time.Millisecond, Volume: 0.6}, // C5
			{Frequency: 659.25, Duration: 120 * time.Millisecond, Volume: 0.6}, // E5
			{Frequency: 783.99, Duration: 250 * time.Millisecond, Volume: 0.7}, // G5
		},
	},
	"error": {
		Name:        "error",
		Description: "Low descending buzz indicating failure",
		Segments: []ToneSegment{
			{Frequency: 400, Duration: 200 * time.Millisecond, Volume: 0.8},
			{Frequency: 300, Duration: 200 * time.Millisecond, Volume: 0.8},
			{Frequency: 200, Duration: 300 * time.Millisecond, Volume: 0.9},
		},
	},
	"info": {
		Name:        "info",
		Description: "Single clean informational beep",
		Segments: []ToneSegment{
			{Frequency: 880, Duration: 200 * time.Millisecond, Volume: 0.5},
		},
	},
	"alert": {
		Name:        "alert",
		Description: "Rapid high-pitched attention signal",
		Segments: []ToneSegment{
			{Frequency: 1200, Duration: 80 * time.Millisecond, Volume: 0.7},
			{Frequency: 0, Duration: 40 * time.Millisecond, Volume: 0},
			{Frequency: 1200, Duration: 80 * time.Millisecond, Volume: 0.7},
			{Frequency: 0, Duration: 40 * time.Millisecond, Volume: 0},
			{Frequency: 1200, Duration: 80 * time.Millisecond, Volume: 0.7},
		},
	},
	"notification": {
		Name:        "notification",
		Description: "Gentle two-note doorbell chime",
		Segments: []ToneSegment{
			{Frequency: 659.25, Duration: 200 * time.Millisecond, Volume: 0.5}, // E5
			{Frequency: 523.25, Duration: 300 * time.Millisecond, Volume: 0.4}, // C5
		},
	},
	"blip": {
		Name:        "blip",
		Description: "Ultra-short confirmation blip",
		Segments: []ToneSegment{
			{Frequency: 1000, Duration: 80 * time.Millisecond, Volume: 0.7},
		},
	},
}

// GeneratePCM produces stereo 16-bit signed little-endian PCM data for the given sound.
func GeneratePCM(def SoundDefinition) []byte {
	totalSamples := 0
	for _, seg := range def.Segments {
		totalSamples += int(float64(SampleRate) * seg.Duration.Seconds())
	}
	// 4 bytes per sample frame (2 channels x 2 bytes per sample)
	buf := make([]byte, 0, totalSamples*4)

	for _, seg := range def.Segments {
		numSamples := int(float64(SampleRate) * seg.Duration.Seconds())
		fadeSamples := SampleRate * 5 / 1000 // 5ms fade in/out

		for i := 0; i < numSamples; i++ {
			t := float64(i) / float64(SampleRate)

			// Envelope to avoid clicks
			envelope := 1.0
			if i < fadeSamples {
				envelope = float64(i) / float64(fadeSamples)
			} else if i > numSamples-fadeSamples {
				envelope = float64(numSamples-i) / float64(fadeSamples)
			}

			var val float64
			if seg.Frequency > 0 {
				val = math.Sin(2*math.Pi*seg.Frequency*t) * seg.Volume * envelope
			}

			sample := int16(val * 32767)
			lo := byte(sample)
			hi := byte(sample >> 8)
			buf = append(buf, lo, hi, lo, hi) // L + R
		}
	}

	return buf
}
