package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildWAV constructs a minimal valid WAV file in memory.
func buildWAV(sampleRate uint32, bitsPerSample, channels uint16, pcmData []byte) []byte {
	dataSize := len(pcmData)
	fmtSize := 16
	fileSize := 4 + (8 + fmtSize) + (8 + dataSize) // WAVE + fmt chunk + data chunk

	buf := make([]byte, 12+8+fmtSize+8+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")

	// fmt chunk
	off := 12
	copy(buf[off:off+4], "fmt ")
	binary.LittleEndian.PutUint32(buf[off+4:off+8], uint32(fmtSize))
	binary.LittleEndian.PutUint16(buf[off+8:off+10], 1) // PCM
	binary.LittleEndian.PutUint16(buf[off+10:off+12], channels)
	binary.LittleEndian.PutUint32(buf[off+12:off+16], sampleRate)
	blockAlign := channels * bitsPerSample / 8
	byteRate := sampleRate * uint32(blockAlign)
	binary.LittleEndian.PutUint32(buf[off+16:off+20], byteRate)
	binary.LittleEndian.PutUint16(buf[off+20:off+22], blockAlign)
	binary.LittleEndian.PutUint16(buf[off+22:off+24], bitsPerSample)

	// data chunk
	off += 8 + fmtSize
	copy(buf[off:off+4], "data")
	binary.LittleEndian.PutUint32(buf[off+4:off+8], uint32(dataSize))
	copy(buf[off+8:], pcmData)

	return buf
}

func putInt16LE(b []byte, v int16) {
	binary.LittleEndian.PutUint16(b, uint16(v))
}

func writeTempWAV(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadWAVStereo16(t *testing.T) {
	// 4 stereo frames at 44100 Hz, 16-bit: known sample values
	pcm := make([]byte, 4*4) // 4 frames * 4 bytes (2ch * 2bytes)
	// Frame 0: L=1000, R=2000
	putInt16LE(pcm[0:2], 1000)
	putInt16LE(pcm[2:4], 2000)
	// Frame 1: L=-1000, R=-2000
	putInt16LE(pcm[4:6], -1000)
	putInt16LE(pcm[6:8], -2000)
	// Frame 2: L=0, R=0
	// Frame 3: L=32767, R=-32768
	putInt16LE(pcm[12:14], 32767)
	putInt16LE(pcm[14:16], -32768)

	wav := buildWAV(44100, 16, 2, pcm)
	path := writeTempWAV(t, wav)

	got, err := LoadWAV(path)
	if err != nil {
		t.Fatalf("LoadWAV: %v", err)
	}

	if len(got) != len(pcm) {
		t.Fatalf("length: got %d, want %d", len(got), len(pcm))
	}

	// Verify samples survive the float64 roundtrip with minimal error
	for i := 0; i < len(pcm); i += 2 {
		want := int16(binary.LittleEndian.Uint16(pcm[i : i+2]))
		gotS := int16(binary.LittleEndian.Uint16(got[i : i+2]))
		diff := want - gotS
		if diff < -1 || diff > 1 {
			t.Errorf("sample at byte %d: got %d, want %d", i, gotS, want)
		}
	}
}

func TestLoadWAVMono(t *testing.T) {
	// 4 mono frames at 44100 Hz, 16-bit
	pcm := make([]byte, 4*2)
	putInt16LE(pcm[0:2], 5000)
	putInt16LE(pcm[2:4], -5000)
	putInt16LE(pcm[4:6], 10000)
	putInt16LE(pcm[6:8], -10000)

	wav := buildWAV(44100, 16, 1, pcm)
	path := writeTempWAV(t, wav)

	got, err := LoadWAV(path)
	if err != nil {
		t.Fatalf("LoadWAV: %v", err)
	}

	// Output should be stereo: 4 frames * 4 bytes
	if len(got) != 4*4 {
		t.Fatalf("length: got %d, want %d", len(got), 4*4)
	}

	// Each frame: L == R (mono duplicated)
	for i := 0; i < 4; i++ {
		left := int16(binary.LittleEndian.Uint16(got[i*4 : i*4+2]))
		right := int16(binary.LittleEndian.Uint16(got[i*4+2 : i*4+4]))
		if left != right {
			t.Errorf("frame %d: L=%d != R=%d", i, left, right)
		}
	}
}

func TestLoadWAVResample(t *testing.T) {
	// 100 mono frames at 22050 Hz → should produce ~200 frames at 44100 Hz
	srcFrames := 100
	pcm := make([]byte, srcFrames*2)
	for i := 0; i < srcFrames; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:(i+1)*2], uint16(int16(i*100)))
	}

	wav := buildWAV(22050, 16, 1, pcm)
	path := writeTempWAV(t, wav)

	got, err := LoadWAV(path)
	if err != nil {
		t.Fatalf("LoadWAV: %v", err)
	}

	// Output is stereo 16-bit: 4 bytes per frame
	outFrames := len(got) / 4
	// Should be approximately 2x the input frames
	if outFrames < 190 || outFrames > 210 {
		t.Errorf("expected ~200 output frames, got %d", outFrames)
	}
}

func TestLoadWAV8Bit(t *testing.T) {
	// 4 mono frames, 8-bit unsigned
	pcm := []byte{
		128, // silence (0)
		255, // max positive
		0,   // max negative
		192, // mid positive
	}

	wav := buildWAV(44100, 8, 1, pcm)
	path := writeTempWAV(t, wav)

	got, err := LoadWAV(path)
	if err != nil {
		t.Fatalf("LoadWAV: %v", err)
	}

	// 4 stereo frames = 16 bytes
	if len(got) != 16 {
		t.Fatalf("length: got %d, want 16", len(got))
	}

	// First sample (128 = silence) should be near 0
	s0 := int16(binary.LittleEndian.Uint16(got[0:2]))
	if s0 < -256 || s0 > 256 {
		t.Errorf("sample 0 (silence): got %d, want ~0", s0)
	}

	// Second sample (255 = max positive) should be strongly positive
	s1 := int16(binary.LittleEndian.Uint16(got[4:6]))
	if s1 < 20000 {
		t.Errorf("sample 1 (max positive): got %d, want > 20000", s1)
	}

	// Third sample (0 = max negative) should be strongly negative
	s2 := int16(binary.LittleEndian.Uint16(got[8:10]))
	if s2 > -20000 {
		t.Errorf("sample 2 (max negative): got %d, want < -20000", s2)
	}
}

func TestLoadWAVInvalidFormat(t *testing.T) {
	// Not a WAV file at all
	path := filepath.Join(t.TempDir(), "notawav.wav")
	if err := os.WriteFile(path, []byte("this is not a wav file, it's just some random text"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWAV(path)
	if err == nil {
		t.Fatal("expected error for non-WAV file")
	}

	// Compressed WAV (format code 6 = A-law)
	compressed := buildWAV(44100, 8, 1, []byte{128, 128})
	compressed[20] = 6 // overwrite format code
	path2 := filepath.Join(t.TempDir(), "compressed.wav")
	if err := os.WriteFile(path2, compressed, 0644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadWAV(path2)
	if err == nil {
		t.Fatal("expected error for compressed WAV")
	}
}
