package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/voice"
)

// --- sendTypes ---

func TestSendTypesContainsExpected(t *testing.T) {
	expected := []string{
		"say", "toast",
		"discord", "discord_voice",
		"slack",
		"telegram", "telegram_audio", "telegram_voice",
	}
	for _, typ := range expected {
		if !sendTypes[typ] {
			t.Errorf("sendTypes missing %q", typ)
		}
	}
}

func TestSendTypesExcludesNonSendable(t *testing.T) {
	excluded := []string{"sound", "webhook", "plugin", "mqtt"}
	for _, typ := range excluded {
		if sendTypes[typ] {
			t.Errorf("sendTypes should not include %q", typ)
		}
	}
}

// --- dryRunVoiceSource ---

func TestDryRunVoiceSourceNonVoiceStep(t *testing.T) {
	for _, typ := range []string{"sound", "toast", "discord", "slack", "telegram", "webhook", "plugin", "mqtt"} {
		s := config.Step{Type: typ, Text: "hello"}
		if got := dryRunVoiceSource(s, nil, ""); got != "" {
			t.Errorf("dryRunVoiceSource(%s) = %q, want empty", typ, got)
		}
	}
}

func TestDryRunVoiceSourceDynamic(t *testing.T) {
	for _, typ := range []string{"say", "discord_voice", "telegram_audio", "telegram_voice"} {
		s := config.Step{Type: typ, Text: "{duration} elapsed"}
		got := dryRunVoiceSource(s, nil, "")
		if got != "(system tts, dynamic)" {
			t.Errorf("dryRunVoiceSource(%s, dynamic) = %q, want \"(system tts, dynamic)\"", typ, got)
		}
	}
}

func TestDryRunVoiceSourceNilCache(t *testing.T) {
	s := config.Step{Type: "say", Text: "hello world"}
	got := dryRunVoiceSource(s, nil, "")
	if got != "(system tts)" {
		t.Errorf("dryRunVoiceSource(nil cache) = %q, want \"(system tts)\"", got)
	}
}

func TestDryRunVoiceSourceCacheMiss(t *testing.T) {
	// Empty cache — no entries to match.
	cache := &voice.Cache{Entries: map[string]voice.CacheEntry{}}
	s := config.Step{Type: "say", Text: "not cached"}
	got := dryRunVoiceSource(s, cache, "nova")
	if got != "(system tts)" {
		t.Errorf("dryRunVoiceSource(cache miss) = %q, want \"(system tts)\"", got)
	}
}

func TestDryRunVoiceSourceDefaultVoiceName(t *testing.T) {
	// When voiceName is empty, should fall back to DefaultVoiceName.
	s := config.Step{Type: "say", Text: "hello world"}
	got := dryRunVoiceSource(s, nil, "")
	// With nil cache it returns "(system tts)" — that's the non-cached path.
	// The voice name fallback only triggers on cache hit, which needs a real cache.
	if got != "(system tts)" {
		t.Errorf("dryRunVoiceSource(empty voice) = %q, want \"(system tts)\"", got)
	}
}

func TestDryRunVoiceSourceAllVoiceTypes(t *testing.T) {
	voiceTypes := []string{"say", "discord_voice", "telegram_audio", "telegram_voice"}
	for _, typ := range voiceTypes {
		s := config.Step{Type: typ, Text: "static text"}
		got := dryRunVoiceSource(s, nil, "nova")
		if got == "" {
			t.Errorf("dryRunVoiceSource(%s) returned empty, want voice source label", typ)
		}
	}
}

// --- loadAndValidate ---

func TestLoadAndValidateValidConfig(t *testing.T) {
	cfgData := map[string]interface{}{
		"config": map[string]interface{}{},
		"profiles": map[string]interface{}{
			"test": map[string]interface{}{
				"ready": map[string]interface{}{
					"steps": []interface{}{
						map[string]interface{}{"type": "say", "text": "hello"},
					},
				},
			},
		},
	}

	path := writeTempConfig(t, cfgData)
	cfg, err := loadAndValidate(path)
	if err != nil {
		t.Fatalf("loadAndValidate: %v", err)
	}
	if _, ok := cfg.Profiles["test"]; !ok {
		t.Error("expected 'test' profile in loaded config")
	}
}

func TestLoadAndValidateInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("{invalid json"), 0644)

	_, err := loadAndValidate(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadAndValidateValidationError(t *testing.T) {
	cfgData := map[string]interface{}{
		"config": map[string]interface{}{
			"default_volume": 200, // out of range
		},
		"profiles": map[string]interface{}{
			"test": map[string]interface{}{
				"ready": map[string]interface{}{
					"steps": []interface{}{
						map[string]interface{}{"type": "say", "text": "hi"},
					},
				},
			},
		},
	}

	path := writeTempConfig(t, cfgData)
	_, err := loadAndValidate(path)
	if err == nil {
		t.Fatal("expected validation error for volume 200")
	}
	if !strings.Contains(err.Error(), "default_volume") {
		t.Errorf("error should mention default_volume, got: %v", err)
	}
}

func TestLoadAndValidateMissingFile(t *testing.T) {
	_, err := loadAndValidate(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadAndValidateEmptySteps(t *testing.T) {
	cfgData := map[string]interface{}{
		"config": map[string]interface{}{},
		"profiles": map[string]interface{}{
			"test": map[string]interface{}{
				"ready": map[string]interface{}{
					"steps": []interface{}{},
				},
			},
		},
	}

	path := writeTempConfig(t, cfgData)
	_, err := loadAndValidate(path)
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
	if !strings.Contains(err.Error(), "no steps") {
		t.Errorf("error should mention 'no steps', got: %v", err)
	}
}

func TestLoadAndValidateUnknownStepType(t *testing.T) {
	cfgData := map[string]interface{}{
		"config": map[string]interface{}{},
		"profiles": map[string]interface{}{
			"test": map[string]interface{}{
				"ready": map[string]interface{}{
					"steps": []interface{}{
						map[string]interface{}{"type": "fax", "text": "hi"},
					},
				},
			},
		},
	}

	path := writeTempConfig(t, cfgData)
	_, err := loadAndValidate(path)
	if err == nil {
		t.Fatal("expected error for unknown step type")
	}
	if !strings.Contains(err.Error(), "fax") {
		t.Errorf("error should mention 'fax', got: %v", err)
	}
}

// writeTempConfig marshals data to a temp JSON file and returns its path.
func writeTempConfig(t *testing.T, data interface{}) string {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "notify-config.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
