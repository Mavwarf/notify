package runner

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

func TestFilterStepsAllRun(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "say", Text: "hi"},
	}
	got := FilterSteps(steps, false, false, 0)
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	got = FilterSteps(steps, true, false, 0)
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

	got := FilterSteps(steps, false, false, 0)
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

	got := FilterSteps(steps, true, false, 0)
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
	got := FilterSteps(nil, false, false, 0)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilterStepsAllFiltered(t *testing.T) {
	steps := []config.Step{
		{Type: "say", Text: "hi", When: "present"},
	}
	got := FilterSteps(steps, true, false, 0)
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
	got := FilterSteps(steps, false, true, 0)
	if len(got) != 2 {
		t.Fatalf("run mode: len = %d, want 2", len(got))
	}
	if got[1].Text != "cmd done" {
		t.Errorf("run mode[1].Text = %q, want %q", got[1].Text, "cmd done")
	}

	// In direct mode: sound + "ready"
	got = FilterSteps(steps, false, false, 0)
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
		name    string
		when    string
		afk     bool
		run     bool
		now     time.Time
		elapsed time.Duration
		want    bool
	}{
		{"empty_always", "", false, false, noon, 0, true},
		{"afk_when_afk", "afk", true, false, noon, 0, true},
		{"afk_when_present", "afk", false, false, noon, 0, false},
		{"present_when_present", "present", false, false, noon, 0, true},
		{"present_when_afk", "present", true, false, noon, 0, false},
		{"run_in_run_mode", "run", false, true, noon, 0, true},
		{"run_in_direct_mode", "run", false, false, noon, 0, false},
		{"direct_in_direct_mode", "direct", false, false, noon, 0, true},
		{"direct_in_run_mode", "direct", false, true, noon, 0, false},
		{"hours_match", "hours:8-22", false, false, noon, 0, true},
		{"hours_no_match", "hours:8-22", false, false, midnight, 0, false},
		{"unknown_skipped", "bogus", false, false, noon, 0, false},
		{"long_match", "long:5m", false, true, noon, 10 * time.Minute, true},
		{"long_below", "long:5m", false, true, noon, 2 * time.Minute, false},
		{"long_exact", "long:5m", false, true, noon, 5 * time.Minute, true},
		{"long_zero_elapsed", "long:5m", false, false, noon, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchWhen(tt.when, tt.afk, tt.run, tt.now, tt.elapsed)
			if got != tt.want {
				t.Errorf("matchWhen(%q, afk=%v, run=%v, elapsed=%v) = %v, want %v",
					tt.when, tt.afk, tt.run, tt.elapsed, got, tt.want)
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

// --- Execute ---

// mockStepExec replaces stepExec for testing and restores it on cleanup.
func mockStepExec(t *testing.T, fn func(config.Step, int, config.Credentials, tmpl.Vars, *int) error) {
	t.Helper()
	orig := stepExec
	t.Cleanup(func() { stepExec = orig })
	stepExec = fn
}

func TestExecuteEmpty(t *testing.T) {
	mockStepExec(t, func(_ config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		t.Fatal("should not be called")
		return nil
	})
	if err := Execute(nil, 80, config.Credentials{}, tmpl.Vars{}, nil); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestExecuteAllParallel(t *testing.T) {
	var mu sync.Mutex
	var ran []string

	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		mu.Lock()
		ran = append(ran, s.Type)
		mu.Unlock()
		return nil
	})

	steps := []config.Step{
		{Type: "discord", Text: "a"},
		{Type: "telegram", Text: "b"},
		{Type: "toast", Message: "c"},
	}
	if err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ran) != 3 {
		t.Errorf("ran %d steps, want 3", len(ran))
	}
}

func TestExecuteAllSequential(t *testing.T) {
	var order []string

	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		order = append(order, s.Type+":"+s.Sound)
		return nil
	})

	steps := []config.Step{
		{Type: "sound", Sound: "first"},
		{Type: "say", Text: "second"},
		{Type: "sound", Sound: "third"},
	}
	if err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil); err != nil {
		t.Fatalf("err = %v", err)
	}
	// Sequential steps must run in order.
	if len(order) != 3 {
		t.Fatalf("ran %d steps, want 3", len(order))
	}
	if order[0] != "sound:first" || order[1] != "say:" || order[2] != "sound:third" {
		t.Errorf("order = %v", order)
	}
}

func TestExecuteMixed(t *testing.T) {
	var mu sync.Mutex
	var ran []string

	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		mu.Lock()
		ran = append(ran, s.Type)
		mu.Unlock()
		return nil
	})

	steps := []config.Step{
		{Type: "discord", Text: "a"},
		{Type: "sound", Sound: "blip"},
		{Type: "telegram", Text: "b"},
		{Type: "say", Text: "hi"},
	}
	if err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ran) != 4 {
		t.Errorf("ran %d steps, want 4", len(ran))
	}
}

func TestExecuteSequentialStopsOnError(t *testing.T) {
	calls := 0
	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		calls++
		if s.Sound == "bad" {
			return errors.New("boom")
		}
		return nil
	})

	steps := []config.Step{
		{Type: "sound", Sound: "ok"},
		{Type: "sound", Sound: "bad"},
		{Type: "sound", Sound: "never"},
	}
	err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want 'boom'", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (should stop after error)", calls)
	}
}

func TestExecuteParallelJoinsErrors(t *testing.T) {
	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		return errors.New(s.Text)
	})

	steps := []config.Step{
		{Type: "discord", Text: "fail-a"},
		{Type: "telegram", Text: "fail-b"},
	}
	err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "fail-a") || !strings.Contains(msg, "fail-b") {
		t.Errorf("err = %v, want both fail-a and fail-b", err)
	}
}

func TestExecuteSeqErrorWaitsForParallel(t *testing.T) {
	var parallelDone sync.WaitGroup
	parallelDone.Add(1)

	mockStepExec(t, func(s config.Step, _ int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		if s.Type == "discord" {
			time.Sleep(50 * time.Millisecond)
			parallelDone.Done()
			return nil
		}
		return errors.New("seq-fail")
	})

	steps := []config.Step{
		{Type: "discord", Text: "slow"},
		{Type: "sound", Sound: "bad"},
	}
	err := Execute(steps, 80, config.Credentials{}, tmpl.Vars{}, nil)
	if err == nil || !strings.Contains(err.Error(), "seq-fail") {
		t.Fatalf("err = %v, want seq-fail", err)
	}
	// If Execute returned before wg.Wait(), this would hang or race.
	parallelDone.Wait()
}

func TestExecutePassesVolume(t *testing.T) {
	var gotVol int
	mockStepExec(t, func(_ config.Step, vol int, _ config.Credentials, _ tmpl.Vars, _ *int) error {
		gotVol = vol
		return nil
	})

	steps := []config.Step{{Type: "sound", Sound: "blip"}}
	Execute(steps, 42, config.Credentials{}, tmpl.Vars{}, nil)
	if gotVol != 42 {
		t.Errorf("volume = %d, want 42", gotVol)
	}
}

func TestMatchLong(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		elapsed time.Duration
		want    bool
	}{
		{"above_threshold", "5m", 10 * time.Minute, true},
		{"exact_threshold", "5m", 5 * time.Minute, true},
		{"below_threshold", "5m", 2 * time.Minute, false},
		{"zero_elapsed", "5m", 0, false},
		{"seconds_spec", "30s", 45 * time.Second, true},
		{"seconds_below", "30s", 10 * time.Second, false},
		{"complex_spec", "1h30m", 2 * time.Hour, true},
		{"complex_below", "1h30m", 1 * time.Hour, false},
		{"invalid_spec", "abc", 10 * time.Minute, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchLong(tt.spec, tt.elapsed)
			if got != tt.want {
				t.Errorf("matchLong(%q, %v) = %v, want %v", tt.spec, tt.elapsed, got, tt.want)
			}
		})
	}
}

func TestFilterStepsLong(t *testing.T) {
	steps := []config.Step{
		{Type: "sound", Sound: "blip"},
		{Type: "discord", Text: "long build", When: "long:5m"},
		{Type: "say", Text: "done"},
	}

	// With 10m elapsed: all 3 steps run.
	got := FilterSteps(steps, false, true, 10*time.Minute)
	if len(got) != 3 {
		t.Errorf("long elapsed: len = %d, want 3", len(got))
	}

	// With 2m elapsed: only 2 steps run (long:5m is skipped).
	got = FilterSteps(steps, false, true, 2*time.Minute)
	if len(got) != 2 {
		t.Errorf("short elapsed: len = %d, want 2", len(got))
	}
	for _, s := range got {
		if s.When == "long:5m" {
			t.Error("short elapsed: long:5m step should not run")
		}
	}

	// With 0 elapsed (direct mode): only 2 steps run.
	got = FilterSteps(steps, false, false, 0)
	if len(got) != 2 {
		t.Errorf("zero elapsed: len = %d, want 2", len(got))
	}
}
