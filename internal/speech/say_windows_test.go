//go:build windows

package speech

import (
	"strings"
	"testing"
)

// --- sayScript ---

func TestSayScriptContainsVolume(t *testing.T) {
	s := sayScript("hello", 75)
	if !strings.Contains(s, "$s.Volume = 75") {
		t.Errorf("script should set volume to 75:\n%s", s)
	}
}

func TestSayScriptContainsText(t *testing.T) {
	s := sayScript("build complete", 50)
	if !strings.Contains(s, "build complete") {
		t.Errorf("script should contain text:\n%s", s)
	}
}

func TestSayScriptEscapesSingleQuotes(t *testing.T) {
	s := sayScript("it's done", 50)
	// PowerShell single-quote escape: ' → ''
	if !strings.Contains(s, "it''s done") {
		t.Errorf("script should escape single quotes:\n%s", s)
	}
}

func TestSayScriptLoadsAssembly(t *testing.T) {
	s := sayScript("test", 50)
	if !strings.Contains(s, "Add-Type -AssemblyName System.Speech") {
		t.Error("script should load System.Speech assembly")
	}
}

func TestSayScriptCreatesSynthesizer(t *testing.T) {
	s := sayScript("test", 50)
	if !strings.Contains(s, "SpeechSynthesizer") {
		t.Error("script should create SpeechSynthesizer")
	}
}

func TestSayScriptZeroVolume(t *testing.T) {
	s := sayScript("muted", 0)
	if !strings.Contains(s, "$s.Volume = 0") {
		t.Errorf("script should set volume to 0:\n%s", s)
	}
}

func TestSayScriptMaxVolume(t *testing.T) {
	s := sayScript("loud", 100)
	if !strings.Contains(s, "$s.Volume = 100") {
		t.Errorf("script should set volume to 100:\n%s", s)
	}
}

// --- sayToFileScript ---

func TestSayToFileScriptContainsPath(t *testing.T) {
	s := sayToFileScript("hello", 50, `C:\temp\out.wav`)
	if !strings.Contains(s, `C:\temp\out.wav`) {
		t.Errorf("script should contain output path:\n%s", s)
	}
}

func TestSayToFileScriptSetsOutputToWave(t *testing.T) {
	s := sayToFileScript("hello", 50, `C:\out.wav`)
	if !strings.Contains(s, "SetOutputToWaveFile") {
		t.Error("script should call SetOutputToWaveFile")
	}
}

func TestSayToFileScriptDisposeSynthesizer(t *testing.T) {
	s := sayToFileScript("hello", 50, `C:\out.wav`)
	if !strings.Contains(s, "$s.Dispose()") {
		t.Error("script should dispose synthesizer")
	}
}

func TestSayToFileScriptEscapesPathQuotes(t *testing.T) {
	s := sayToFileScript("hello", 50, `C:\it's a path\out.wav`)
	// PowerShell: ' → ''
	if !strings.Contains(s, `C:\it''s a path\out.wav`) {
		t.Errorf("script should escape path quotes:\n%s", s)
	}
}

func TestSayToFileScriptEscapesTextQuotes(t *testing.T) {
	s := sayToFileScript("it's done", 50, `C:\out.wav`)
	if !strings.Contains(s, "it''s done") {
		t.Errorf("script should escape text quotes:\n%s", s)
	}
}

func TestSayToFileScriptVolume(t *testing.T) {
	s := sayToFileScript("test", 30, `C:\out.wav`)
	if !strings.Contains(s, "$s.Volume = 30") {
		t.Errorf("script should set volume to 30:\n%s", s)
	}
}
