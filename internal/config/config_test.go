package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnmarshalBasic(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": {
					"steps": [
						{"type": "sound", "sound": "success"},
						{"type": "say", "text": "Ready!"}
					]
				}
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.AFKThresholdSeconds != DefaultAFKThreshold {
		t.Errorf("AFKThresholdSeconds = %d, want %d", cfg.Options.AFKThresholdSeconds, DefaultAFKThreshold)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
	p := cfg.Profiles["default"]
	if len(p) != 1 {
		t.Fatalf("len(default) = %d, want 1", len(p))
	}
	act := p["ready"]
	if len(act.Steps) != 2 {
		t.Fatalf("len(steps) = %d, want 2", len(act.Steps))
	}
	if act.Steps[0].Type != "sound" || act.Steps[0].Sound != "success" {
		t.Errorf("step 0 = %+v", act.Steps[0])
	}
	if act.Steps[1].Type != "say" || act.Steps[1].Text != "Ready!" {
		t.Errorf("step 1 = %+v", act.Steps[1])
	}
}

func TestUnmarshalAFKThreshold(t *testing.T) {
	data := []byte(`{
		"config": { "afk_threshold_seconds": 600 },
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.AFKThresholdSeconds != 600 {
		t.Errorf("AFKThresholdSeconds = %d, want 600", cfg.Options.AFKThresholdSeconds)
	}
	if len(cfg.Profiles) != 1 {
		t.Errorf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
}

func TestUnmarshalDefaultVolume(t *testing.T) {
	data := []byte(`{
		"config": { "default_volume": 75 },
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.DefaultVolume != 75 {
		t.Errorf("DefaultVolume = %d, want 75", cfg.Options.DefaultVolume)
	}
	if len(cfg.Profiles) != 1 {
		t.Errorf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
}

func TestUnmarshalDefaultVolumeOmitted(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.DefaultVolume != DefaultVolume {
		t.Errorf("DefaultVolume = %d, want %d", cfg.Options.DefaultVolume, DefaultVolume)
	}
}

func TestUnmarshalWhenField(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": {
					"steps": [
						{"type": "sound", "sound": "success"},
						{"type": "say", "text": "Hi", "when": "present"},
						{"type": "toast", "message": "Hi", "when": "afk"}
					]
				}
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	steps := cfg.Profiles["default"]["ready"].Steps
	if steps[0].When != "" {
		t.Errorf("step 0 When = %q, want empty", steps[0].When)
	}
	if steps[1].When != "present" {
		t.Errorf("step 1 When = %q, want present", steps[1].When)
	}
	if steps[2].When != "afk" {
		t.Errorf("step 2 When = %q, want afk", steps[2].When)
	}
}

func TestUnmarshalVolume(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": {
					"steps": [
						{"type": "sound", "sound": "blip", "volume": 50},
						{"type": "sound", "sound": "blip"}
					]
				}
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	steps := cfg.Profiles["default"]["ready"].Steps
	if steps[0].Volume == nil || *steps[0].Volume != 50 {
		t.Errorf("step 0 Volume = %v, want 50", steps[0].Volume)
	}
	if steps[1].Volume != nil {
		t.Errorf("step 1 Volume = %v, want nil", steps[1].Volume)
	}
}

func TestUnmarshalCredentials(t *testing.T) {
	data := []byte(`{
		"config": {
			"credentials": {
				"discord_webhook": "https://discord.com/api/webhooks/123/abc"
			}
		},
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "discord", "text": "Ready!"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.Credentials.DiscordWebhook != "https://discord.com/api/webhooks/123/abc" {
		t.Errorf("DiscordWebhook = %q", cfg.Options.Credentials.DiscordWebhook)
	}
	if len(cfg.Profiles) != 1 {
		t.Errorf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
}

func TestUnmarshalNoCredentials(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.Credentials.DiscordWebhook != "" {
		t.Errorf("DiscordWebhook = %q, want empty", cfg.Options.Credentials.DiscordWebhook)
	}
}

func TestUnmarshalCooldown(t *testing.T) {
	data := []byte(`{
		"config": { "cooldown": true },
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !cfg.Options.Cooldown {
		t.Error("Cooldown = false, want true")
	}
}

func TestUnmarshalDefaultCooldownSeconds(t *testing.T) {
	data := []byte(`{
		"config": { "cooldown": true, "cooldown_seconds": 15 },
		"profiles": {
			"default": {
				"done": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Options.CooldownSeconds != 15 {
		t.Errorf("CooldownSeconds = %d, want 15", cfg.Options.CooldownSeconds)
	}
}

func TestUnmarshalCooldownSeconds(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": {
					"cooldown_seconds": 30,
					"steps": [{"type": "sound", "sound": "success"}]
				}
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	act := cfg.Profiles["default"]["ready"]
	if act.CooldownSeconds != 30 {
		t.Errorf("CooldownSeconds = %d, want 30", act.CooldownSeconds)
	}
}

// --- Validate tests ---

func TestValidateValidConfig(t *testing.T) {
	cfg := Config{
		Options: Options{
			AFKThresholdSeconds: 300,
			DefaultVolume:       80,
			CooldownSeconds:     30,
		},
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{
					Steps: []Step{
						{Type: "sound", Sound: "blip"},
						{Type: "say", Text: "Ready!", When: "present"},
						{Type: "toast", Message: "Done", When: "afk"},
						{Type: "discord", Text: "Done", When: "hours:8-22"},
					},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateUnknownStepType(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{Steps: []Step{{Type: "email", Text: "hi"}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown step type")
	}
	if !strings.Contains(err.Error(), `unknown type "email"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateUnknownWhenCondition(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip", When: "never"}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown when condition")
	}
	if !strings.Contains(err.Error(), `unknown when condition "never"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateInvalidHoursSpec(t *testing.T) {
	tests := []struct {
		when string
	}{
		{"hours:abc"},
		{"hours:25-8"},
		{"hours:8"},
	}
	for _, tt := range tests {
		cfg := Config{
			Profiles: map[string]Profile{
				"default": {
					"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip", When: tt.when}}},
				},
			},
		}
		if err := Validate(cfg); err == nil {
			t.Errorf("expected error for when=%q", tt.when)
		}
	}
}

func TestValidateValidHoursSpec(t *testing.T) {
	tests := []string{"hours:0-23", "hours:8-22", "hours:22-8"}
	for _, when := range tests {
		cfg := Config{
			Profiles: map[string]Profile{
				"default": {
					"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip", When: when}}},
				},
			},
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("expected valid for when=%q, got: %v", when, err)
		}
	}
}

func TestValidateMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want string
	}{
		{"sound without sound", Step{Type: "sound"}, "requires \"sound\" field"},
		{"say without text", Step{Type: "say"}, "requires \"text\" field"},
		{"discord without text", Step{Type: "discord"}, "requires \"text\" field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Profiles: map[string]Profile{
					"default": {"ready": Action{Steps: []Step{tt.step}}},
				},
			}
			err := Validate(cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected %q in error, got: %v", tt.want, err)
			}
		})
	}
}

func TestValidateEmptyAction(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {"ready": Action{Steps: []Step{}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty action")
	}
	if !strings.Contains(err.Error(), "has no steps") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateVolumeOutOfRange(t *testing.T) {
	vol := 150
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip", Volume: &vol}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for volume out of range")
	}
	if !strings.Contains(err.Error(), "volume 150 out of range") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDefaultVolumeOutOfRange(t *testing.T) {
	cfg := Config{
		Options: Options{DefaultVolume: 200},
		Profiles: map[string]Profile{
			"default": {"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip"}}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for default_volume out of range")
	}
	if !strings.Contains(err.Error(), "default_volume 200 out of range") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	cfg := Config{
		Options: Options{DefaultVolume: 200},
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{Steps: []Step{{Type: "bogus", When: "never"}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected errors")
	}
	// Should report all three: default_volume, unknown type, unknown when.
	msg := err.Error()
	if !strings.Contains(msg, "default_volume") {
		t.Errorf("missing default_volume error in: %s", msg)
	}
	if !strings.Contains(msg, "unknown type") {
		t.Errorf("missing unknown type error in: %s", msg)
	}
	if !strings.Contains(msg, "unknown when") {
		t.Errorf("missing unknown when error in: %s", msg)
	}
}

func TestResolveDirectMatch(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"boss": {
				"ready": Action{Steps: []Step{{Type: "sound", Sound: "success"}}},
			},
		},
	}

	act, err := Resolve(cfg, "boss", "ready")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(act.Steps) != 1 || act.Steps[0].Sound != "success" {
		t.Errorf("unexpected action: %+v", act)
	}
}

func TestResolveFallbackToDefault(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				"ready": Action{Steps: []Step{{Type: "sound", Sound: "blip"}}},
			},
			"boss": {},
		},
	}

	act, err := Resolve(cfg, "boss", "ready")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if act.Steps[0].Sound != "blip" {
		t.Errorf("expected fallback to default, got %+v", act)
	}
}

func TestResolveNotFound(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {},
		},
	}

	_, err := Resolve(cfg, "boss", "ready")
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}
