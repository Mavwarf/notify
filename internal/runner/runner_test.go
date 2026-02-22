package runner

import (
	"errors"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/config"
)

func TestFilterStepsAllRun(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "say", Text: "hi"},
	}
	got := FilterSteps(steps, false, false)
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	got = FilterSteps(steps, true, false)
	if len(got) != 2 {
		t.Errorf("len = %d, want 2 (no when = always run)", len(got))
	}
}

func TestFilterStepsPresent(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "say", Text: "hi", When: "present"},
		{Type: "toast", Message: "afk msg", When: "afk"},
	}

	got := FilterSteps(steps, false, false)
	if len(got) != 2 {
		t.Fatalf("present: len = %d, want 2", len(got))
	}
	if got[0].Type != "sound" {
		t.Errorf("present[0].Type = %q", got[0].Type)
	}
	if got[1].Type != "say" {
		t.Errorf("present[1].Type = %q", got[1].Type)
	}
}

func TestFilterStepsAFK(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "say", Text: "hi", When: "present"},
		{Type: "toast", Message: "afk msg", When: "afk"},
	}

	got := FilterSteps(steps, true, false)
	if len(got) != 2 {
		t.Fatalf("afk: len = %d, want 2", len(got))
	}
	if got[0].Type != "sound" {
		t.Errorf("afk[0].Type = %q", got[0].Type)
	}
	if got[1].Type != "toast" {
		t.Errorf("afk[1].Type = %q", got[1].Type)
	}
}

func TestFilterStepsEmpty(t *testing.T) {
	got := FilterSteps(nil, false, false)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilterStepsAllFiltered(t *testing.T) {
	steps := []config.Step{
		{Type: "say", Text: "hi", When: "present"},
	}
	got := FilterSteps(steps, true, false)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (all filtered when afk)", len(got))
	}
}

func TestFilterStepsRun(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "say", Text: "cmd done", When: "run"},
		{Type: "say", Text: "ready", When: "direct"},
	}

	// In run mode: sound + "cmd done"
	got := FilterSteps(steps, false, true)
	if len(got) != 2 {
		t.Fatalf("run mode: len = %d, want 2", len(got))
	}
	if got[1].Text != "cmd done" {
		t.Errorf("run mode[1].Text = %q, want %q", got[1].Text, "cmd done")
	}

	// In direct mode: sound + "ready"
	got = FilterSteps(steps, false, false)
	if len(got) != 2 {
		t.Fatalf("direct mode: len = %d, want 2", len(got))
	}
	if got[1].Text != "ready" {
		t.Errorf("direct mode[1].Text = %q, want %q", got[1].Text, "ready")
	}
}

func TestMatchHours(t *testing.T) {
	tests := []struct {
		name string
		spec string
		hour int
		want bool
	}{
		// Daytime range 8-22
		{"daytime_inside", "8-22", 12, true},
		{"daytime_start", "8-22", 8, true},
		{"daytime_before_end", "8-22", 21, true},
		{"daytime_at_end", "8-22", 22, false},
		{"daytime_before_start", "8-22", 7, false},
		{"daytime_midnight", "8-22", 0, false},

		// Cross-midnight range 22-8
		{"night_at_start", "22-8", 22, true},
		{"night_late", "22-8", 23, true},
		{"night_midnight", "22-8", 0, true},
		{"night_early", "22-8", 5, true},
		{"night_at_end", "22-8", 8, false},
		{"night_midday", "22-8", 12, false},

		// Same values → zero-width, never matches
		{"same_8_8", "8-8", 8, false},
		{"same_0_0", "0-0", 0, false},

		// Full day 0-24 is invalid (24 out of range)
		{"invalid_24", "0-24", 12, false},
		{"invalid_negative", "-1-8", 5, false},
		{"invalid_letters", "abc-def", 10, false},
		{"invalid_no_dash", "822", 10, false},
		{"invalid_empty", "", 10, false},

		// Boundary hours
		{"boundary_0_start", "0-6", 0, true},
		{"boundary_23_end", "20-23", 23, false},
		{"boundary_23_inside", "20-23", 22, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 1, 15, tt.hour, 30, 0, 0, time.Local)
			got := matchHours(tt.spec, now)
			if got != tt.want {
				t.Errorf("matchHours(%q, hour=%d) = %v, want %v", tt.spec, tt.hour, got, tt.want)
			}
		})
	}
}

func TestMatchWhen(t *testing.T) {
	noon := time.Date(2026, 1, 15, 12, 0, 0, 0, time.Local)
	midnight := time.Date(2026, 1, 15, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name string
		when string
		afk  bool
		run  bool
		now  time.Time
		want bool
	}{
		{"empty_always", "", false, false, noon, true},
		{"afk_when_afk", "afk", true, false, noon, true},
		{"afk_when_present", "afk", false, false, noon, false},
		{"present_when_present", "present", false, false, noon, true},
		{"present_when_afk", "present", true, false, noon, false},
		{"run_in_run_mode", "run", false, true, noon, true},
		{"run_in_direct_mode", "run", false, false, noon, false},
		{"direct_in_direct_mode", "direct", false, false, noon, true},
		{"direct_in_run_mode", "direct", false, true, noon, false},
		{"hours_match", "hours:8-22", false, false, noon, true},
		{"hours_no_match", "hours:8-22", false, false, midnight, false},
		{"unknown_skipped", "bogus", false, false, noon, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchWhen(tt.when, tt.afk, tt.run, tt.now)
			if got != tt.want {
				t.Errorf("matchWhen(%q, afk=%v, run=%v) = %v, want %v",
					tt.when, tt.afk, tt.run, got, tt.want)
			}
		})
	}
}

func TestRetryOnceSuccess(t *testing.T) {
	calls := 0
	err := retryOnce(func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryOnceFailThenSuccess(t *testing.T) {
	calls := 0
	err := retryOnce(func() error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestRetryOnceBothFail(t *testing.T) {
	calls := 0
	err := retryOnce(func() error {
		calls++
		return errors.New("fail")
	})
	if err == nil {
		t.Error("err = nil, want error")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}
