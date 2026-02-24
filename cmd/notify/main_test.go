package main

import (
	"fmt"
	"testing"
	"time"

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

// --- formatDuration ---

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0ms"},
		{500 * time.Millisecond, "500ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1s"},
		{3 * time.Second, "3s"},
		{65 * time.Second, "1m5s"},
		{2*time.Minute + 15*time.Second, "2m15s"},
		{1*time.Hour + 30*time.Minute, "1h30m0s"},
		{1*time.Second + 499*time.Millisecond, "1s"},   // rounds down
		{1*time.Second + 500*time.Millisecond, "2s"},   // rounds up
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// --- formatDurationSay ---

func TestFormatDurationSay(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "less than a second"},
		{500 * time.Millisecond, "less than a second"},
		{1 * time.Second, "1 second"},
		{2 * time.Second, "2 seconds"},
		{60 * time.Second, "1 minute"},
		{61 * time.Second, "1 minute and 1 second"},
		{2*time.Minute + 15*time.Second, "2 minutes and 15 seconds"},
		{1 * time.Hour, "1 hour"},
		{1*time.Hour + 1*time.Second, "1 hour and 1 second"},
		{1*time.Hour + 30*time.Minute + 5*time.Second, "1 hour, 30 minutes and 5 seconds"},
		{2*time.Hour + 1*time.Minute, "2 hours and 1 minute"},
	}
	for _, tt := range tests {
		if got := formatDurationSay(tt.d); got != tt.want {
			t.Errorf("formatDurationSay(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// --- pluralize ---

func TestPluralize(t *testing.T) {
	tests := []struct {
		n         int
		singular  string
		plural    string
		want      string
	}{
		{0, "second", "seconds", "0 seconds"},
		{1, "second", "seconds", "1 second"},
		{2, "second", "seconds", "2 seconds"},
		{1, "hour", "hours", "1 hour"},
		{42, "minute", "minutes", "42 minutes"},
	}
	for _, tt := range tests {
		if got := pluralize(tt.n, tt.singular, tt.plural); got != tt.want {
			t.Errorf("pluralize(%d, %q, %q) = %q, want %q",
				tt.n, tt.singular, tt.plural, got, tt.want)
		}
	}
}

// --- resolveMatchAction ---

func TestResolveMatchActionFirstWins(t *testing.T) {
	matches := []matchPair{
		{"FAIL", "error"},
		{"passed", "ready"},
	}
	output := "3 failed, 47 passed\nFAIL"
	if got := resolveMatchAction(matches, output); got != "error" {
		t.Errorf("resolveMatchAction() = %q, want \"error\"", got)
	}
}

func TestResolveMatchActionSecondMatch(t *testing.T) {
	matches := []matchPair{
		{"FAIL", "error"},
		{"passed", "ready"},
	}
	output := "All tests passed"
	if got := resolveMatchAction(matches, output); got != "ready" {
		t.Errorf("resolveMatchAction() = %q, want \"ready\"", got)
	}
}

func TestResolveMatchActionNoMatch(t *testing.T) {
	matches := []matchPair{
		{"FAIL", "error"},
		{"passed", "ready"},
	}
	output := "something else entirely"
	if got := resolveMatchAction(matches, output); got != "" {
		t.Errorf("resolveMatchAction() = %q, want \"\"", got)
	}
}

func TestResolveMatchActionEmpty(t *testing.T) {
	if got := resolveMatchAction(nil, "anything"); got != "" {
		t.Errorf("resolveMatchAction(nil, ...) = %q, want \"\"", got)
	}
}

func TestResolveMatchActionEmptyOutput(t *testing.T) {
	matches := []matchPair{{"FAIL", "error"}}
	if got := resolveMatchAction(matches, ""); got != "" {
		t.Errorf("resolveMatchAction(matches, \"\") = %q, want \"\"", got)
	}
}

// --- lastNLines ---

func TestLastNLinesMoreThanN(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5\n"
	if got := lastNLines(input, 3); got != "line3\nline4\nline5" {
		t.Errorf("lastNLines(5 lines, 3) = %q, want \"line3\\nline4\\nline5\"", got)
	}
}

func TestLastNLinesExactN(t *testing.T) {
	input := "line1\nline2\nline3"
	if got := lastNLines(input, 3); got != "line1\nline2\nline3" {
		t.Errorf("lastNLines(3 lines, 3) = %q, want \"line1\\nline2\\nline3\"", got)
	}
}

func TestLastNLinesFewerThanN(t *testing.T) {
	input := "line1\nline2"
	if got := lastNLines(input, 5); got != "line1\nline2" {
		t.Errorf("lastNLines(2 lines, 5) = %q, want \"line1\\nline2\"", got)
	}
}

func TestLastNLinesEmpty(t *testing.T) {
	if got := lastNLines("", 5); got != "" {
		t.Errorf("lastNLines(\"\", 5) = %q, want \"\"", got)
	}
}

func TestLastNLinesSingleLine(t *testing.T) {
	if got := lastNLines("hello\n", 1); got != "hello" {
		t.Errorf("lastNLines(\"hello\\n\", 1) = %q, want \"hello\"", got)
	}
}

func TestLastNLinesTrailingNewlines(t *testing.T) {
	input := "a\nb\nc\n\n"
	if got := lastNLines(input, 2); got != "b\nc" {
		t.Errorf("lastNLines(%q, 2) = %q, want \"b\\nc\"", input, got)
	}
}
