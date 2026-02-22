package audio

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

var (
	otoCtx     *oto.Context
	otoOnce    sync.Once
	otoInitErr error
)

func getContext() (*oto.Context, error) {
	otoOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   SampleRate,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		}
		var readyChan chan struct{}
		otoCtx, readyChan, otoInitErr = oto.NewContext(op)
		if otoInitErr == nil {
			<-readyChan
		}
	})
	return otoCtx, otoInitErr
}

// Play plays a sound, blocking until playback completes. If name matches a
// built-in sound, plays the generated tone; otherwise treats name as a WAV
// file path. volume is a multiplier from 0.0 (silent) to 1.0 (full volume).
func Play(name string, volume float64) error {
	var pcm []byte
	if def, ok := Sounds[name]; ok {
		pcm = GeneratePCM(def)
	} else {
		var err error
		pcm, err = LoadWAV(name)
		if err != nil {
			return err
		}
	}
	applyVolume16(pcm, volume)
	return playStereo16(pcm)
}

// applyVolume16 scales 16-bit signed little-endian PCM samples by the given volume.
func applyVolume16(data []byte, volume float64) {
	if volume >= 1.0 {
		return
	}
	for i := 0; i+1 < len(data); i += 2 {
		sample := int16(data[i]) | int16(data[i+1])<<8
		sample = int16(float64(sample) * volume)
		data[i] = byte(sample)
		data[i+1] = byte(sample >> 8)
	}
}

// playStereo16 plays 44100 Hz stereo 16-bit signed LE PCM through the shared context.
func playStereo16(pcm []byte) error {
	ctx, err := getContext()
	if err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}

	player := ctx.NewPlayer(bytes.NewReader(pcm))
	player.Play()

	for player.IsPlaying() {
		time.Sleep(5 * time.Millisecond)
	}

	return player.Close()
}
