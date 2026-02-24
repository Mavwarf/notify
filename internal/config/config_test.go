package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// p is a shorthand for constructing Profile with Actions in tests.
func p(actions map[string]Action) Profile {
	return Profile{Actions: actions}
}

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
	prof := cfg.Profiles["default"]
	if len(prof.Actions) != 1 {
		t.Fatalf("len(default.Actions) = %d, want 1", len(prof.Actions))
	}
	act := prof.Actions["ready"]
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

	steps := cfg.Profiles["default"].Actions["ready"].Steps
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

	steps := cfg.Profiles["default"].Actions["ready"].Steps
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

func TestUnmarshalLog(t *testing.T) {
	data := []byte(`{
		"config": { "log": true },
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

	if !cfg.Options.Log {
		t.Error("Log = false, want true")
	}
}

func TestUnmarshalLogDefault(t *testing.T) {
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

	if cfg.Options.Log {
		t.Error("Log = true, want false (default)")
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

	act := cfg.Profiles["default"].Actions["ready"]
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
			Credentials: Credentials{
				DiscordWebhook: "https://example.com",
				SlackWebhook:   "https://hooks.slack.com/services/T/B/X",
				TelegramToken:  "tok",
				TelegramChatID: "123",
			},
		},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready": {
					Steps: []Step{
						{Type: "sound", Sound: "blip"},
						{Type: "say", Text: "Ready!", When: "present"},
						{Type: "toast", Message: "Done", When: "afk"},
						{Type: "discord", Text: "Done", When: "hours:8-22"},
						{Type: "discord_voice", Text: "Done", When: "afk"},
						{Type: "slack", Text: "Done", When: "afk"},
						{Type: "telegram", Text: "Done", When: "afk"},
						{Type: "telegram_audio", Text: "Done", When: "afk"},
						{Type: "telegram_voice", Text: "Done", When: "afk"},
					},
				},
			}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateUnknownStepType(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "email", Text: "hi"}}},
			}),
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

func TestValidateNeverCondition(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "blip", When: "never"}}},
			}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config with when=never, got: %v", err)
	}
}

func TestValidateUnknownWhenCondition(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "blip", When: "bogus"}}},
			}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown when condition")
	}
	if !strings.Contains(err.Error(), `unknown when condition "bogus"`) {
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
				"default": p(map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "blip", When: tt.when}}},
				}),
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
				"default": p(map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "blip", When: when}}},
				}),
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
		{"toast without message", Step{Type: "toast"}, "requires \"message\" field"},
		{"discord without text", Step{Type: "discord"}, "requires \"text\" field"},
		{"discord_voice without text", Step{Type: "discord_voice"}, "requires \"text\" field"},
		{"slack without text", Step{Type: "slack"}, "requires \"text\" field"},
		{"telegram without text", Step{Type: "telegram"}, "requires \"text\" field"},
		{"telegram_audio without text", Step{Type: "telegram_audio"}, "requires \"text\" field"},
		{"telegram_voice without text", Step{Type: "telegram_voice"}, "requires \"text\" field"},
		{"webhook without url", Step{Type: "webhook", Text: "hi"}, "requires \"url\" field"},
		{"webhook without text", Step{Type: "webhook", URL: "https://example.com"}, "requires \"text\" field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Profiles: map[string]Profile{
					"default": p(map[string]Action{"ready": {Steps: []Step{tt.step}}}),
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
			"default": p(map[string]Action{"ready": {Steps: []Step{}}}),
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
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "blip", Volume: &vol}}},
			}),
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
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
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
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "bogus", When: "bogus"}}},
			}),
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

func TestValidateDiscordMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "discord", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing discord credentials")
	}
	if !strings.Contains(err.Error(), "credentials.discord_webhook") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDiscordWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{DiscordWebhook: "https://example.com"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "discord", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateDiscordVoiceMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "discord_voice", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing discord credentials")
	}
	if !strings.Contains(err.Error(), "credentials.discord_webhook") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDiscordVoiceWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{DiscordWebhook: "https://example.com"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "discord_voice", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateSlackMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "slack", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing slack credentials")
	}
	if !strings.Contains(err.Error(), "credentials.slack_webhook") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSlackWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{SlackWebhook: "https://hooks.slack.com/services/T/B/X"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "slack", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateTelegramMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing telegram credentials")
	}
	if !strings.Contains(err.Error(), "credentials.telegram_token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTelegramPartialCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{TelegramToken: "tok"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing telegram_chat_id")
	}
}

func TestValidateTelegramWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{TelegramToken: "tok", TelegramChatID: "123"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateTelegramAudioMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram_audio", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing telegram credentials")
	}
	if !strings.Contains(err.Error(), "credentials.telegram_token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTelegramAudioWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{TelegramToken: "tok", TelegramChatID: "123"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram_audio", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateTelegramVoiceMissingCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram_voice", Text: "hi"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing telegram credentials")
	}
	if !strings.Contains(err.Error(), "credentials.telegram_token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTelegramVoiceWithCredentials(t *testing.T) {
	cfg := Config{
		Options: Options{Credentials: Credentials{TelegramToken: "tok", TelegramChatID: "123"}},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "telegram_voice", Text: "hi"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

// --- expandEnvCredentials tests ---

func TestExpandEnvCredentials(t *testing.T) {
	t.Setenv("NOTIFY_TEST_DISCORD", "https://discord.com/api/webhooks/123/abc")
	t.Setenv("NOTIFY_TEST_SLACK", "https://hooks.slack.com/services/T/B/X")
	t.Setenv("NOTIFY_TEST_TG_TOKEN", "bot123:AAHxx")
	t.Setenv("NOTIFY_TEST_TG_CHAT", "99999")

	cfg := Config{
		Options: Options{
			Credentials: Credentials{
				DiscordWebhook: "$NOTIFY_TEST_DISCORD",
				SlackWebhook:   "${NOTIFY_TEST_SLACK}",
				TelegramToken:  "$NOTIFY_TEST_TG_TOKEN",
				TelegramChatID: "${NOTIFY_TEST_TG_CHAT}",
			},
		},
	}
	expandEnvCredentials(&cfg)

	if cfg.Options.Credentials.DiscordWebhook != "https://discord.com/api/webhooks/123/abc" {
		t.Errorf("DiscordWebhook = %q", cfg.Options.Credentials.DiscordWebhook)
	}
	if cfg.Options.Credentials.SlackWebhook != "https://hooks.slack.com/services/T/B/X" {
		t.Errorf("SlackWebhook = %q", cfg.Options.Credentials.SlackWebhook)
	}
	if cfg.Options.Credentials.TelegramToken != "bot123:AAHxx" {
		t.Errorf("TelegramToken = %q", cfg.Options.Credentials.TelegramToken)
	}
	if cfg.Options.Credentials.TelegramChatID != "99999" {
		t.Errorf("TelegramChatID = %q", cfg.Options.Credentials.TelegramChatID)
	}
}

func TestExpandEnvUndefined(t *testing.T) {
	cfg := Config{
		Options: Options{
			Credentials: Credentials{
				DiscordWebhook: "$NOTIFY_TEST_UNDEFINED_VAR",
			},
		},
	}
	expandEnvCredentials(&cfg)

	if cfg.Options.Credentials.DiscordWebhook != "" {
		t.Errorf("DiscordWebhook = %q, want empty for undefined var", cfg.Options.Credentials.DiscordWebhook)
	}
}

func TestExpandEnvLiteral(t *testing.T) {
	cfg := Config{
		Options: Options{
			Credentials: Credentials{
				DiscordWebhook: "https://discord.com/api/webhooks/123/abc",
				SlackWebhook:   "https://hooks.slack.com/services/T/B/X",
				TelegramToken:  "bot123:AAHxx",
				TelegramChatID: "99999",
			},
		},
	}
	expandEnvCredentials(&cfg)

	if cfg.Options.Credentials.DiscordWebhook != "https://discord.com/api/webhooks/123/abc" {
		t.Errorf("DiscordWebhook = %q", cfg.Options.Credentials.DiscordWebhook)
	}
	if cfg.Options.Credentials.SlackWebhook != "https://hooks.slack.com/services/T/B/X" {
		t.Errorf("SlackWebhook = %q", cfg.Options.Credentials.SlackWebhook)
	}
	if cfg.Options.Credentials.TelegramToken != "bot123:AAHxx" {
		t.Errorf("TelegramToken = %q", cfg.Options.Credentials.TelegramToken)
	}
	if cfg.Options.Credentials.TelegramChatID != "99999" {
		t.Errorf("TelegramChatID = %q", cfg.Options.Credentials.TelegramChatID)
	}
}

func TestUnmarshalExitCodes(t *testing.T) {
	data := []byte(`{
		"config": { "exit_codes": { "2": "warning", "130": "cancelled" } },
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(cfg.Options.ExitCodes) != 2 {
		t.Fatalf("len(ExitCodes) = %d, want 2", len(cfg.Options.ExitCodes))
	}
	if cfg.Options.ExitCodes["2"] != "warning" {
		t.Errorf("ExitCodes[\"2\"] = %q, want \"warning\"", cfg.Options.ExitCodes["2"])
	}
	if cfg.Options.ExitCodes["130"] != "cancelled" {
		t.Errorf("ExitCodes[\"130\"] = %q, want \"cancelled\"", cfg.Options.ExitCodes["130"])
	}
}

func TestValidateExitCodesValid(t *testing.T) {
	cfg := Config{
		Options: Options{
			ExitCodes: map[string]string{"0": "success", "2": "warning", "130": "cancelled"},
		},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateExitCodesInvalidKey(t *testing.T) {
	cfg := Config{
		Options: Options{
			ExitCodes: map[string]string{"abc": "warning"},
		},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for non-integer key")
	}
	if !strings.Contains(err.Error(), "not a valid integer") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExitCodesEmptyValue(t *testing.T) {
	cfg := Config{
		Options: Options{
			ExitCodes: map[string]string{"2": ""},
		},
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty action value")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveDirectMatch(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"boss": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
			}),
		},
	}

	name, act, err := Resolve(cfg, "boss", "ready")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "boss" {
		t.Errorf("resolved name = %q, want %q", name, "boss")
	}
	if len(act.Steps) != 1 || act.Steps[0].Sound != "success" {
		t.Errorf("unexpected action: %+v", act)
	}
}

func TestResolveFallbackToDefault(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}},
			}),
			"boss": p(map[string]Action{}),
		},
	}

	_, act, err := Resolve(cfg, "boss", "ready")
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
			"default": p(map[string]Action{}),
		},
	}

	_, _, err := Resolve(cfg, "boss", "ready")
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

// --- Profile inheritance tests ---

func TestUnmarshalExtends(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "sound", "sound": "success"}] }
			},
			"quiet": {
				"extends": "default",
				"ready": { "steps": [{"type": "sound", "sound": "blip", "volume": 30}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	q := cfg.Profiles["quiet"]
	if q.Extends != "default" {
		t.Errorf("Extends = %q, want %q", q.Extends, "default")
	}
	if len(q.Actions) != 1 {
		t.Errorf("len(quiet.Actions) = %d, want 1 (before resolve)", len(q.Actions))
	}
}

func TestUnmarshalNoExtends(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "sound", "sound": "success"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Profiles["default"].Extends != "" {
		t.Errorf("Extends = %q, want empty", cfg.Profiles["default"].Extends)
	}
}

func TestResolveInheritance(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{
				"ready":   {Steps: []Step{{Type: "sound", Sound: "success"}}},
				"error":   {Steps: []Step{{Type: "sound", Sound: "error"}}},
				"warning": {Steps: []Step{{Type: "sound", Sound: "warning"}}},
			}),
			"quiet": {
				Extends: "default",
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}},
				},
			},
		},
	}

	if err := resolveInheritance(&cfg); err != nil {
		t.Fatalf("resolveInheritance: %v", err)
	}

	q := cfg.Profiles["quiet"]
	// Should have all three actions: ready (overridden), error and warning (inherited).
	if len(q.Actions) != 3 {
		t.Fatalf("len(quiet.Actions) = %d, want 3", len(q.Actions))
	}
	if q.Actions["ready"].Steps[0].Sound != "blip" {
		t.Errorf("quiet.ready should be overridden, got sound=%q", q.Actions["ready"].Steps[0].Sound)
	}
	if q.Actions["error"].Steps[0].Sound != "error" {
		t.Errorf("quiet.error should be inherited, got sound=%q", q.Actions["error"].Steps[0].Sound)
	}
	if q.Actions["warning"].Steps[0].Sound != "warning" {
		t.Errorf("quiet.warning should be inherited, got sound=%q", q.Actions["warning"].Steps[0].Sound)
	}
}

func TestResolveInheritanceChain(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"base": p(map[string]Action{
				"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				"error": {Steps: []Step{{Type: "sound", Sound: "error"}}},
			}),
			"mid": {
				Extends: "base",
				Actions: map[string]Action{
					"warning": {Steps: []Step{{Type: "sound", Sound: "warning"}}},
				},
			},
			"leaf": {
				Extends: "mid",
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}},
				},
			},
		},
	}

	if err := resolveInheritance(&cfg); err != nil {
		t.Fatalf("resolveInheritance: %v", err)
	}

	leaf := cfg.Profiles["leaf"]
	if len(leaf.Actions) != 3 {
		t.Fatalf("len(leaf.Actions) = %d, want 3", len(leaf.Actions))
	}
	if leaf.Actions["ready"].Steps[0].Sound != "blip" {
		t.Error("leaf.ready should be overridden")
	}
	if leaf.Actions["error"].Steps[0].Sound != "error" {
		t.Error("leaf.error should be inherited from base")
	}
	if leaf.Actions["warning"].Steps[0].Sound != "warning" {
		t.Error("leaf.warning should be inherited from mid")
	}
}

func TestResolveInheritanceCircular(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"a": {
				Extends: "b",
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
			"b": {
				Extends: "a",
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}

	err := resolveInheritance(&cfg)
	if err == nil {
		t.Fatal("expected error for circular extends")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular error, got: %v", err)
	}
}

func TestResolveInheritanceUnknownParent(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"quiet": {
				Extends: "nonexistent",
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}

	err := resolveInheritance(&cfg)
	if err == nil {
		t.Fatal("expected error for unknown parent")
	}
	if !strings.Contains(err.Error(), "unknown profile") {
		t.Errorf("expected unknown profile error, got: %v", err)
	}
}

func TestResolveInheritanceSelfExtend(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"a": {
				Extends: "a",
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}

	err := resolveInheritance(&cfg)
	if err == nil {
		t.Fatal("expected error for self-extending profile")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular error, got: %v", err)
	}
}

// --- Profile alias tests ---

func TestUnmarshalAliases(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"myproject": {
				"aliases": ["mp", "proj"],
				"ready": { "steps": [{"type": "sound", "sound": "success"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	p := cfg.Profiles["myproject"]
	if len(p.Aliases) != 2 {
		t.Fatalf("len(Aliases) = %d, want 2", len(p.Aliases))
	}
	if p.Aliases[0] != "mp" || p.Aliases[1] != "proj" {
		t.Errorf("Aliases = %v, want [mp proj]", p.Aliases)
	}
}

func TestUnmarshalNoAliases(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "sound", "sound": "success"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(cfg.Profiles["default"].Aliases) != 0 {
		t.Errorf("Aliases = %v, want empty", cfg.Profiles["default"].Aliases)
	}
}

func TestResolveAlias(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"myproject": {
				Aliases: []string{"mp", "proj"},
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				},
			},
		},
	}

	name, act, err := Resolve(cfg, "mp", "ready")
	if err != nil {
		t.Fatalf("Resolve via alias: %v", err)
	}
	if name != "myproject" {
		t.Errorf("resolved name = %q, want %q", name, "myproject")
	}
	if act.Steps[0].Sound != "success" {
		t.Errorf("unexpected action: %+v", act)
	}

	name, act, err = Resolve(cfg, "proj", "ready")
	if err != nil {
		t.Fatalf("Resolve via alias: %v", err)
	}
	if name != "myproject" {
		t.Errorf("resolved name = %q, want %q", name, "myproject")
	}
	if act.Steps[0].Sound != "success" {
		t.Errorf("unexpected action: %+v", act)
	}
}

func TestResolveAliasDefaultFallback(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				Actions: map[string]Action{
					"error": {Steps: []Step{{Type: "sound", Sound: "error"}}},
				},
			},
			"myproject": {
				Aliases: []string{"mp"},
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				},
			},
		},
	}

	// Action not in aliased profile, should fall back to default.
	name, act, err := Resolve(cfg, "mp", "error")
	if err != nil {
		t.Fatalf("Resolve alias with default fallback: %v", err)
	}
	if name != "myproject" {
		t.Errorf("resolved name = %q, want %q", name, "myproject")
	}
	if act.Steps[0].Sound != "error" {
		t.Errorf("expected default fallback, got %+v", act)
	}
}

func TestValidateAliasShadowsProfile(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": {
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
			"myproject": {
				Aliases: []string{"default"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for alias shadowing profile name")
	}
	if !strings.Contains(err.Error(), "shadows an existing profile") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- MergeCredentials tests ---

func TestMergeCredentialsNilProfile(t *testing.T) {
	global := Credentials{
		DiscordWebhook: "https://discord.com/global",
		SlackWebhook:   "https://slack.com/global",
		TelegramToken:  "globaltoken",
		TelegramChatID: "globalchat",
	}
	merged := MergeCredentials(global, nil)
	if merged != global {
		t.Errorf("expected global unchanged, got %+v", merged)
	}
}

func TestMergeCredentialsPartialOverride(t *testing.T) {
	global := Credentials{
		DiscordWebhook: "https://discord.com/global",
		SlackWebhook:   "https://slack.com/global",
		TelegramToken:  "globaltoken",
		TelegramChatID: "globalchat",
	}
	profile := &Credentials{
		DiscordWebhook: "https://discord.com/override",
	}
	merged := MergeCredentials(global, profile)
	if merged.DiscordWebhook != "https://discord.com/override" {
		t.Errorf("DiscordWebhook = %q, want override", merged.DiscordWebhook)
	}
	if merged.SlackWebhook != "https://slack.com/global" {
		t.Errorf("SlackWebhook = %q, want global", merged.SlackWebhook)
	}
	if merged.TelegramToken != "globaltoken" {
		t.Errorf("TelegramToken = %q, want global", merged.TelegramToken)
	}
	if merged.TelegramChatID != "globalchat" {
		t.Errorf("TelegramChatID = %q, want global", merged.TelegramChatID)
	}
}

func TestMergeCredentialsFullOverride(t *testing.T) {
	global := Credentials{
		DiscordWebhook: "https://discord.com/global",
		SlackWebhook:   "https://slack.com/global",
		TelegramToken:  "globaltoken",
		TelegramChatID: "globalchat",
	}
	profile := &Credentials{
		DiscordWebhook: "https://discord.com/override",
		SlackWebhook:   "https://slack.com/override",
		TelegramToken:  "overridetoken",
		TelegramChatID: "overridechat",
	}
	merged := MergeCredentials(global, profile)
	if merged != *profile {
		t.Errorf("expected full override, got %+v", merged)
	}
}

// --- Per-profile credentials tests ---

func TestUnmarshalProfileCredentials(t *testing.T) {
	data := []byte(`{
		"config": {
			"credentials": {
				"discord_webhook": "https://discord.com/global"
			}
		},
		"profiles": {
			"projectA": {
				"credentials": {
					"discord_webhook": "https://discord.com/project-a"
				},
				"done": { "steps": [{"type": "discord", "text": "Done!"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	p := cfg.Profiles["projectA"]
	if p.Credentials == nil {
		t.Fatal("expected profile credentials, got nil")
	}
	if p.Credentials.DiscordWebhook != "https://discord.com/project-a" {
		t.Errorf("profile DiscordWebhook = %q", p.Credentials.DiscordWebhook)
	}
	if cfg.Options.Credentials.DiscordWebhook != "https://discord.com/global" {
		t.Errorf("global DiscordWebhook = %q", cfg.Options.Credentials.DiscordWebhook)
	}
}

func TestUnmarshalProfileNoCredentials(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"default": {
				"ready": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Profiles["default"].Credentials != nil {
		t.Errorf("expected nil credentials, got %+v", cfg.Profiles["default"].Credentials)
	}
}

func TestValidateProfileCredentialsOverrideGlobal(t *testing.T) {
	// Profile has discord_webhook but global does not — should pass.
	cfg := Config{
		Profiles: map[string]Profile{
			"projectA": {
				Credentials: &Credentials{DiscordWebhook: "https://discord.com/project-a"},
				Actions: map[string]Action{
					"done": {Steps: []Step{{Type: "discord", Text: "Done!"}}},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid with profile credentials, got: %v", err)
	}
}

func TestValidateProfileCredentialsMissingBoth(t *testing.T) {
	// Neither global nor profile has discord_webhook — should fail.
	cfg := Config{
		Profiles: map[string]Profile{
			"projectA": {
				Actions: map[string]Action{
					"done": {Steps: []Step{{Type: "discord", Text: "Done!"}}},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing discord credentials")
	}
	if !strings.Contains(err.Error(), "credentials.discord_webhook") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExpandEnvProfileCredentials(t *testing.T) {
	t.Setenv("NOTIFY_TEST_PROFILE_DISCORD", "https://discord.com/profile-env")

	cfg := Config{
		Profiles: map[string]Profile{
			"projectA": {
				Credentials: &Credentials{
					DiscordWebhook: "$NOTIFY_TEST_PROFILE_DISCORD",
				},
				Actions: map[string]Action{
					"done": {Steps: []Step{{Type: "sound", Sound: "blip"}}},
				},
			},
		},
	}
	expandEnvCredentials(&cfg)

	if cfg.Profiles["projectA"].Credentials.DiscordWebhook != "https://discord.com/profile-env" {
		t.Errorf("profile DiscordWebhook = %q", cfg.Profiles["projectA"].Credentials.DiscordWebhook)
	}
}

func TestResolveInheritanceCredentials(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"base": {
				Credentials: &Credentials{
					DiscordWebhook: "https://discord.com/base",
					SlackWebhook:   "https://slack.com/base",
				},
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				},
			},
			"child": {
				Extends: "base",
				Credentials: &Credentials{
					DiscordWebhook: "https://discord.com/child",
				},
				Actions: map[string]Action{},
			},
		},
	}

	if err := resolveInheritance(&cfg); err != nil {
		t.Fatalf("resolveInheritance: %v", err)
	}

	child := cfg.Profiles["child"]
	if child.Credentials == nil {
		t.Fatal("expected merged credentials, got nil")
	}
	if child.Credentials.DiscordWebhook != "https://discord.com/child" {
		t.Errorf("child DiscordWebhook = %q, want child override", child.Credentials.DiscordWebhook)
	}
	if child.Credentials.SlackWebhook != "https://slack.com/base" {
		t.Errorf("child SlackWebhook = %q, want base inherited", child.Credentials.SlackWebhook)
	}
}

func TestResolveInheritanceCredentialsChildOnly(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"base": {
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				},
			},
			"child": {
				Extends: "base",
				Credentials: &Credentials{
					DiscordWebhook: "https://discord.com/child",
				},
				Actions: map[string]Action{},
			},
		},
	}

	if err := resolveInheritance(&cfg); err != nil {
		t.Fatalf("resolveInheritance: %v", err)
	}

	child := cfg.Profiles["child"]
	if child.Credentials == nil {
		t.Fatal("expected child credentials preserved, got nil")
	}
	if child.Credentials.DiscordWebhook != "https://discord.com/child" {
		t.Errorf("child DiscordWebhook = %q", child.Credentials.DiscordWebhook)
	}
}

func TestResolveInheritanceCredentialsParentOnly(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"base": {
				Credentials: &Credentials{
					DiscordWebhook: "https://discord.com/base",
				},
				Actions: map[string]Action{
					"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}},
				},
			},
			"child": {
				Extends: "base",
				Actions: map[string]Action{},
			},
		},
	}

	if err := resolveInheritance(&cfg); err != nil {
		t.Fatalf("resolveInheritance: %v", err)
	}

	child := cfg.Profiles["child"]
	if child.Credentials == nil {
		t.Fatal("expected inherited credentials, got nil")
	}
	if child.Credentials.DiscordWebhook != "https://discord.com/base" {
		t.Errorf("child DiscordWebhook = %q, want inherited from base", child.Credentials.DiscordWebhook)
	}
}

func TestValidateDuplicateAlias(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"project1": {
				Aliases: []string{"p"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
			"project2": {
				Aliases: []string{"p"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate alias")
	}
	if !strings.Contains(err.Error(), "already claimed by") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Match rule tests ---

func TestUnmarshalMatch(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"work": {
				"match": { "dir": "/work/" },
				"ready": { "steps": [{"type": "sound", "sound": "success"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	prof := cfg.Profiles["work"]
	if prof.Match == nil {
		t.Fatal("Match is nil")
	}
	if prof.Match.Dir != "/work/" {
		t.Errorf("Match.Dir = %q, want /work/", prof.Match.Dir)
	}
	if prof.Match.Env != "" {
		t.Errorf("Match.Env = %q, want empty", prof.Match.Env)
	}
	if len(prof.Actions) != 1 {
		t.Errorf("len(Actions) = %d, want 1", len(prof.Actions))
	}
}

func TestUnmarshalMatchDirAndEnv(t *testing.T) {
	data := []byte(`{
		"profiles": {
			"team": {
				"match": { "dir": "/team/", "env": "TEAM=alpha" },
				"ready": { "steps": [{"type": "sound", "sound": "blip"}] }
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	prof := cfg.Profiles["team"]
	if prof.Match == nil {
		t.Fatal("Match is nil")
	}
	if prof.Match.Dir != "/team/" {
		t.Errorf("Match.Dir = %q, want /team/", prof.Match.Dir)
	}
	if prof.Match.Env != "TEAM=alpha" {
		t.Errorf("Match.Env = %q, want TEAM=alpha", prof.Match.Env)
	}
}

func TestMatchProfileDir(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"work": {
				Match:   &MatchRule{Dir: "/work/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	got := MatchProfile(cfg, "/home/user/work/project")
	if got != "work" {
		t.Errorf("MatchProfile = %q, want work", got)
	}
}

func TestMatchProfileDirWindows(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"work": {
				Match:   &MatchRule{Dir: "/work/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	// Backslash path should be normalized to forward slash for matching.
	got := MatchProfile(cfg, `C:\Users\me\work\project`)
	if got != "work" {
		t.Errorf("MatchProfile = %q, want work", got)
	}
}

func TestMatchProfileEnv(t *testing.T) {
	t.Setenv("NOTIFY_TEST_TEAM", "alpha")

	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"team": {
				Match:   &MatchRule{Env: "NOTIFY_TEST_TEAM=alpha"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	got := MatchProfile(cfg, "/some/dir")
	if got != "team" {
		t.Errorf("MatchProfile = %q, want team", got)
	}
}

func TestMatchProfileEnvEmpty(t *testing.T) {
	t.Setenv("NOTIFY_TEST_EMPTY", "")

	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"empty": {
				Match:   &MatchRule{Env: "NOTIFY_TEST_EMPTY="},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	got := MatchProfile(cfg, "/some/dir")
	if got != "empty" {
		t.Errorf("MatchProfile = %q, want empty", got)
	}
}

func TestMatchProfileAND(t *testing.T) {
	t.Setenv("NOTIFY_TEST_AND", "yes")

	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"both": {
				Match:   &MatchRule{Dir: "/project/", Env: "NOTIFY_TEST_AND=yes"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	// Both match.
	if got := MatchProfile(cfg, "/home/project/foo"); got != "both" {
		t.Errorf("both match: got %q, want both", got)
	}

	// Dir matches but env doesn't.
	t.Setenv("NOTIFY_TEST_AND", "no")
	if got := MatchProfile(cfg, "/home/project/foo"); got != "default" {
		t.Errorf("env mismatch: got %q, want default", got)
	}

	// Env matches but dir doesn't.
	t.Setenv("NOTIFY_TEST_AND", "yes")
	if got := MatchProfile(cfg, "/home/other/foo"); got != "default" {
		t.Errorf("dir mismatch: got %q, want default", got)
	}
}

func TestMatchProfileNoMatch(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"work": {
				Match:   &MatchRule{Dir: "/work/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
		},
	}

	got := MatchProfile(cfg, "/home/personal/stuff")
	if got != "default" {
		t.Errorf("MatchProfile = %q, want default", got)
	}
}

func TestMatchProfileAlphabetical(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"beta": {
				Match:   &MatchRule{Dir: "/shared/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}},
			},
			"alpha": {
				Match:   &MatchRule{Dir: "/shared/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "info"}}}},
			},
		},
	}

	got := MatchProfile(cfg, "/home/shared/project")
	if got != "alpha" {
		t.Errorf("MatchProfile = %q, want alpha (alphabetical tiebreaker)", got)
	}
}

func TestMatchProfileNilMatch(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}}),
			"noMatch": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}}),
		},
	}

	// Profile without match rule should never be selected.
	got := MatchProfile(cfg, "/anything")
	if got != "default" {
		t.Errorf("MatchProfile = %q, want default", got)
	}
}

func TestValidateMatchEmpty(t *testing.T) {
	cfg := Config{
		Options: Options{DefaultVolume: 100, AFKThresholdSeconds: 300},
		Profiles: map[string]Profile{
			"bad": {
				Match:   &MatchRule{},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty match rule")
	}
	if !strings.Contains(err.Error(), "must have at least one condition") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMatchBadEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"no equals", "JUSTKEY"},
		{"empty key", "=value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Options: Options{DefaultVolume: 100, AFKThresholdSeconds: 300},
				Profiles: map[string]Profile{
					"bad": {
						Match:   &MatchRule{Env: tt.env},
						Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
					},
				},
			}
			err := Validate(cfg)
			if err == nil {
				t.Fatal("expected error for bad env")
			}
			if !strings.Contains(err.Error(), "match env must be KEY=VALUE") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateOutputLinesValid(t *testing.T) {
	cfg := Config{
		Options:  Options{DefaultVolume: 100, OutputLines: 10},
		Profiles: map[string]Profile{"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}})},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateOutputLinesNegative(t *testing.T) {
	cfg := Config{
		Options:  Options{DefaultVolume: 100, OutputLines: -1},
		Profiles: map[string]Profile{"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}})},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative output_lines")
	}
	if !strings.Contains(err.Error(), "output_lines") {
		t.Errorf("error should mention output_lines: %v", err)
	}
}

func TestValidateOutputLinesTooLarge(t *testing.T) {
	cfg := Config{
		Options:  Options{DefaultVolume: 100, OutputLines: 1001},
		Profiles: map[string]Profile{"default": p(map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "success"}}}})},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for output_lines > 1000")
	}
	if !strings.Contains(err.Error(), "output_lines") {
		t.Errorf("error should mention output_lines: %v", err)
	}
}

func TestValidateMatchGood(t *testing.T) {
	cfg := Config{
		Options: Options{DefaultVolume: 100, AFKThresholdSeconds: 300},
		Profiles: map[string]Profile{
			"work": {
				Match:   &MatchRule{Dir: "/work/"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
			"team": {
				Match:   &MatchRule{Env: "TEAM=alpha"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
			"both": {
				Match:   &MatchRule{Dir: "/project/", Env: "ENV=prod"},
				Actions: map[string]Action{"ready": {Steps: []Step{{Type: "sound", Sound: "blip"}}}},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
