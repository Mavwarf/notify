package main

import (
	"fmt"
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

func TestDetectAFKIdle(t *testing.T) {
	orig := idleFunc
	t.Cleanup(func() { idleFunc = orig })

	idleFunc = func() (float64, error) { return 600, nil }
	cfg := config.Config{Options: config.Options{AFKThresholdSeconds: 300}}
	if !detectAFK(cfg) {
		t.Error("expected AFK when idle 600s >= threshold 300s")
	}
}

func TestDetectAFKPresent(t *testing.T) {
	orig := idleFunc
	t.Cleanup(func() { idleFunc = orig })

	idleFunc = func() (float64, error) { return 10, nil }
	cfg := config.Config{Options: config.Options{AFKThresholdSeconds: 300}}
	if detectAFK(cfg) {
		t.Error("expected present when idle 10s < threshold 300s")
	}
}

func TestResolveExitActionMapped(t *testing.T) {
	codes := map[string]string{"2": "warning", "130": "cancelled"}
	if got := resolveExitAction(codes, 2); got != "warning" {
		t.Errorf("resolveExitAction(codes, 2) = %q, want \"warning\"", got)
	}
	if got := resolveExitAction(codes, 130); got != "cancelled" {
		t.Errorf("resolveExitAction(codes, 130) = %q, want \"cancelled\"", got)
	}
}

func TestResolveExitActionDefaultReady(t *testing.T) {
	codes := map[string]string{"2": "warning"}
	if got := resolveExitAction(codes, 0); got != "ready" {
		t.Errorf("resolveExitAction(codes, 0) = %q, want \"ready\"", got)
	}
}

func TestResolveExitActionDefaultError(t *testing.T) {
	codes := map[string]string{"2": "warning"}
	if got := resolveExitAction(codes, 1); got != "error" {
		t.Errorf("resolveExitAction(codes, 1) = %q, want \"error\"", got)
	}
}

func TestResolveExitActionNilMap(t *testing.T) {
	if got := resolveExitAction(nil, 0); got != "ready" {
		t.Errorf("resolveExitAction(nil, 0) = %q, want \"ready\"", got)
	}
	if got := resolveExitAction(nil, 1); got != "error" {
		t.Errorf("resolveExitAction(nil, 1) = %q, want \"error\"", got)
	}
}

func TestResolveExitActionOverrideZero(t *testing.T) {
	codes := map[string]string{"0": "done"}
	if got := resolveExitAction(codes, 0); got != "done" {
		t.Errorf("resolveExitAction(codes, 0) = %q, want \"done\"", got)
	}
}

func TestDetectAFKErrorFailsOpen(t *testing.T) {
	orig := idleFunc
	t.Cleanup(func() { idleFunc = orig })

	idleFunc = func() (float64, error) { return 0, fmt.Errorf("no idle detection") }
	cfg := config.Config{Options: config.Options{AFKThresholdSeconds: 300}}
	if detectAFK(cfg) {
		t.Error("expected present (fail-open) on error")
	}
}
