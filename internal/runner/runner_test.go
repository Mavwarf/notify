package runner

import (
	"testing"

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
