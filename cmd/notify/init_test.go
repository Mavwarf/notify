package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mavwarf/notify/internal/config"
)

func TestBuildInitConfigMinimal(t *testing.T) {
	cfg := buildInitConfig(false, false, false, false,
		config.Credentials{}, true, 300, nil)

	if !cfg.Options.Log {
		t.Error("expected log to be true")
	}
	if cfg.Options.AFKThresholdSeconds != 300 {
		t.Errorf("expected AFK threshold 300, got %d", cfg.Options.AFKThresholdSeconds)
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Fatal("expected default profile")
	}
	action := cfg.Profiles["default"].Actions["ready"]
	// Minimal: sound + say only (no toast, discord, slack, telegram).
	if len(action.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(action.Steps))
	}
}

func TestBuildInitConfigAllChannels(t *testing.T) {
	creds := config.Credentials{
		DiscordWebhook: "https://discord.com/api/webhooks/test",
		SlackWebhook:   "https://hooks.slack.com/test",
		TelegramToken:  "123:ABC",
		TelegramChatID: "456",
	}
	cfg := buildInitConfig(true, true, true, true,
		creds, false, 600, nil)

	if cfg.Options.Log {
		t.Error("expected log to be false")
	}

	action := cfg.Profiles["default"].Actions["ready"]
	// sound + say + toast + discord + slack + telegram = 6.
	if len(action.Steps) != 6 {
		t.Errorf("expected 6 steps, got %d", len(action.Steps))
	}

	// Verify step types.
	types := make([]string, len(action.Steps))
	for i, s := range action.Steps {
		types[i] = s.Type
	}
	want := []string{"sound", "say", "toast", "discord", "slack", "telegram"}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("step[%d] type = %q, want %q", i, types[i], w)
		}
	}
}

func TestBuildInitConfigExtraProfiles(t *testing.T) {
	extra := []initProfile{
		{name: "webapp", dir: "/home/user/webapp"},
		{name: "api"},
	}
	cfg := buildInitConfig(false, false, false, false,
		config.Credentials{}, true, 300, extra)

	if _, ok := cfg.Profiles["webapp"]; !ok {
		t.Error("expected webapp profile")
	}
	if cfg.Profiles["webapp"].Extends != "default" {
		t.Errorf("expected webapp to extend default, got %q", cfg.Profiles["webapp"].Extends)
	}
	if cfg.Profiles["webapp"].Match == nil || cfg.Profiles["webapp"].Match.Dir != "/home/user/webapp" {
		t.Error("expected webapp match dir")
	}

	if _, ok := cfg.Profiles["api"]; !ok {
		t.Error("expected api profile")
	}
	if cfg.Profiles["api"].Match != nil {
		t.Error("expected api to have no match rule")
	}
}

func TestBuildInitConfigActions(t *testing.T) {
	cfg := buildInitConfig(false, false, false, false,
		config.Credentials{}, true, 300, nil)

	for _, name := range []string{"ready", "error", "done", "attention"} {
		if _, ok := cfg.Profiles["default"].Actions[name]; !ok {
			t.Errorf("expected action %q in default profile", name)
		}
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.json")

	cfg := buildInitConfig(false, false, false, false,
		config.Credentials{}, true, 300, nil)

	if err := writeConfig(path, cfg); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	// Must be valid JSON.
	var parsed config.Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}

	if _, ok := parsed.Profiles["default"]; !ok {
		t.Error("expected default profile in written config")
	}
}

func TestWriteConfigOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.json")

	cfg1 := buildInitConfig(false, false, false, false,
		config.Credentials{}, false, 100, nil)
	cfg2 := buildInitConfig(true, false, false, false,
		config.Credentials{}, true, 500, nil)

	if err := writeConfig(path, cfg1); err != nil {
		t.Fatalf("first writeConfig: %v", err)
	}
	if err := writeConfig(path, cfg2); err != nil {
		t.Fatalf("second writeConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var parsed config.Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if !parsed.Options.Log {
		t.Error("expected log=true from second write")
	}
}
