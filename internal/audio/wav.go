package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// maxWAVSize is the maximum WAV file size we'll load (50 MB).
const maxWAVSize = 50 * 1024 * 1024

// LoadWAV reads a WAV file and returns raw stereo 16-bit signed LE PCM at 44100 Hz.
// Supports PCM format (format code 1) with 8-bit, 16-bit, or 24-bit samples,
// mono or stereo. Resamples to 44100 Hz via linear interpolation if needed.
func LoadWAV(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}
	if len(data) > maxWAVSize {
		return nil, fmt.Errorf("wav: file too large (%d bytes, max %d)", len(data), maxWAVSize)
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav: file too short")
	}

	// RIFF header
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav: not a WAV file")
	}

	// Find fmt chunk
	fmtOff, fmtSize, err := findChunk(data, "fmt ")
	if err != nil {
		return nil, err
	}
	if fmtSize < 16 {
		return nil, fmt.Errorf("wav: fmt chunk too short")
	}

	format := binary.LittleEndian.Uint16(data[fmtOff : fmtOff+2])
	if format != 1 {
		return nil, fmt.Errorf("wav: unsupported format %d (only PCM supported)", format)
	}
	channels := binary.LittleEndian.Uint16(data[fmtOff+2 : fmtOff+4])
	sampleRate := binary.LittleEndian.Uint32(data[fmtOff+4 : fmtOff+8])
	bitsPerSample := binary.LittleEndian.Uint16(data[fmtOff+14 : fmtOff+16])

	if channels < 1 || channels > 2 {
		return nil, fmt.Errorf("wav: unsupported channel count %d", channels)
	}
	if bitsPerSample != 8 && bitsPerSample != 16 && bitsPerSample != 24 {
		return nil, fmt.Errorf("wav: unsupported bit depth %d", bitsPerSample)
	}

	// Find data chunk
	dataOff, dataSize, err := findChunk(data, "data")
	if err != nil {
		return nil, err
	}
	if dataOff+dataSize > len(data) {
		dataSize = len(data) - dataOff
	}

	raw := data[dataOff : dataOff+dataSize]

	// Decode to float64 samples interleaved by channel
	bytesPerSample := int(bitsPerSample) / 8
	frameSize := bytesPerSample * int(channels)
	if frameSize == 0 {
		return nil, fmt.Errorf("wav: invalid frame size")
	}
	numFrames := len(raw) / frameSize
	if numFrames == 0 {
		return nil, fmt.Errorf("wav: no audio data")
	}

	// Decode all frames to stereo float64 pairs
	samples := make([]float64, numFrames*2) // stereo pairs
	for i := 0; i < numFrames; i++ {
		off := i * frameSize
		var left, right float64

		left = decodeSample(raw, off, bitsPerSample)
		if channels == 2 {
			right = decodeSample(raw, off+bytesPerSample, bitsPerSample)
		} else {
			right = left
		}

		samples[i*2] = left
		samples[i*2+1] = right
	}

	// Resample to 44100 Hz if needed
	if sampleRate != SampleRate {
		samples = resampleLinear(samples, int(sampleRate), SampleRate)
		numFrames = len(samples) / 2
	}

	// Convert to 16-bit LE PCM bytes
	pcm := make([]byte, numFrames*4) // 2 channels * 2 bytes
	for i := 0; i < numFrames; i++ {
		left := clamp16(samples[i*2])
		right := clamp16(samples[i*2+1])
		pcm[i*4] = byte(left)
		pcm[i*4+1] = byte(left >> 8)
		pcm[i*4+2] = byte(right)
		pcm[i*4+3] = byte(right >> 8)
	}

	return pcm, nil
}

// findChunk locates a RIFF chunk by its 4-byte ID and returns (dataOffset, dataSize).
func findChunk(data []byte, id string) (int, int, error) {
	off := 12 // skip RIFF header
	for off+8 <= len(data) {
		chunkID := string(data[off : off+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if chunkID == id {
			dataStart := off + 8
			return dataStart, chunkSize, nil
		}
		// Advance to next chunk (chunks are word-aligned)
		off += 8 + chunkSize
		if off%2 != 0 {
			off++
		}
	}
	return 0, 0, fmt.Errorf("wav: %q chunk not found", id)
}

// decodeSample reads one sample at the given byte offset and returns it as float64 in [-1, 1].
func decodeSample(data []byte, off int, bitsPerSample uint16) float64 {
	switch bitsPerSample {
	case 8:
		// 8-bit WAV is unsigned (0-255, 128 = silence)
		return (float64(data[off]) - 128.0) / 128.0
	case 16:
		// 16-bit WAV is signed little-endian
		s := int16(data[off]) | int16(data[off+1])<<8
		return float64(s) / 32768.0
	case 24:
		// 24-bit WAV is signed little-endian
		val := int(data[off]) | int(data[off+1])<<8 | int(data[off+2])<<16
		if val >= 1<<23 {
			val -= 1 << 24
		}
		return float64(val) / 8388608.0
	}
	return 0
}

// resampleLinear resamples stereo float64 pairs from srcRate to dstRate using linear interpolation.
func resampleLinear(samples []float64, srcRate, dstRate int) []float64 {
	srcFrames := len(samples) / 2
	ratio := float64(srcRate) / float64(dstRate)
	dstFrames := int(math.Ceil(float64(srcFrames) / ratio))
	out := make([]float64, dstFrames*2)

	for i := 0; i < dstFrames; i++ {
		srcPos := float64(i) * ratio
		idx := int(srcPos)
		frac := srcPos - float64(idx)

		if idx+1 < srcFrames {
			out[i*2] = samples[idx*2]*(1-frac) + samples[(idx+1)*2]*frac
			out[i*2+1] = samples[idx*2+1]*(1-frac) + samples[(idx+1)*2+1]*frac
		} else if idx < srcFrames {
			out[i*2] = samples[idx*2]
			out[i*2+1] = samples[idx*2+1]
		}
	}

	return out
}

// clamp16 converts a float64 in [-1, 1] to int16, clamping to avoid overflow.
func clamp16(f float64) int16 {
	s := f * 32767.0
	if s > 32767 {
		return 32767
	}
	if s < -32768 {
		return -32768
	}
	return int16(s)
}
