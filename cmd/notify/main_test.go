package main

import (
	"testing"

	"github.com/Mavwarf/notify/internal/config"
)

func TestResolveVolumeCLIOverride(t *testing.T) {
	cfg := config.Config{Options: config.Options{DefaultVolume: 80}}
	if got := resolveVolume(50, cfg); got != 50 {
		t.Errorf("resolveVolume(50, cfg) = %d, want 50", got)
	}
}

func TestResolveVolumeFallsBackToConfig(t *testing.T) {
	cfg := config.Config{Options: config.Options{DefaultVolume: 80}}
	if got := resolveVolume(-1, cfg); got != 80 {
		t.Errorf("resolveVolume(-1, cfg) = %d, want 80", got)
	}
}

func TestResolveCooldownPerAction(t *testing.T) {
	act := &config.Action{CooldownSeconds: 10}
	cfg := config.Config{Options: config.Options{CooldownSeconds: 60}}

	enabled, sec := resolveCooldown(act, cfg, true)
	if !enabled {
		t.Error("expected cooldown enabled with flag=true")
	}
	if sec != 10 {
		t.Errorf("expected per-action cooldown 10, got %d", sec)
	}
}

func TestResolveCooldownFallsBackToGlobal(t *testing.T) {
	act := &config.Action{CooldownSeconds: 0}
	cfg := config.Config{Options: config.Options{Cooldown: true, CooldownSeconds: 60}}

	enabled, sec := resolveCooldown(act, cfg, false)
	if !enabled {
		t.Error("expected cooldown enabled via config")
	}
	if sec != 60 {
		t.Errorf("expected global cooldown 60, got %d", sec)
	}
}

func TestResolveCooldownDisabled(t *testing.T) {
	act := &config.Action{}
	cfg := config.Config{}

	enabled, _ := resolveCooldown(act, cfg, false)
	if enabled {
		t.Error("expected cooldown disabled")
	}
}

func TestShouldLogConfigEnabled(t *testing.T) {
	cfg := config.Config{Options: config.Options{Log: true}}
	if !shouldLog(cfg, false) {
		t.Error("expected true when config.Log is true")
	}
}

func TestShouldLogFlagEnabled(t *testing.T) {
	cfg := config.Config{}
	if !shouldLog(cfg, true) {
		t.Error("expected true when flag is true")
	}
}

func TestShouldLogBothDisabled(t *testing.T) {
	cfg := config.Config{}
	if shouldLog(cfg, false) {
		t.Error("expected false when both disabled")
	}
}
