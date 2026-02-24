package eventlog

import (
	"testing"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

func TestStepSummarySound(t *testing.T) {
	s := config.Step{Type: "sound", Sound: "blip"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	want := "sound=blip"
	if got != want {
		t.Errorf("StepSummary(sound) = %q, want %q", got, want)
	}
}

func TestStepSummarySay(t *testing.T) {
	s := config.Step{Type: "say", Text: "{profile} ready"}
	vars := tmpl.Vars{Profile: "boss"}
	got := StepSummary(s, &vars)
	want := `text="boss ready"`
	if got != want {
		t.Errorf("StepSummary(say) = %q, want %q", got, want)
	}
}

func TestStepSummaryToast(t *testing.T) {
	s := config.Step{Type: "toast", Title: "Build", Message: "{Profile} done"}
	vars := tmpl.Vars{Profile: "boss"}
	got := StepSummary(s, &vars)
	want := `title="Build"  message="Boss done"`
	if got != want {
		t.Errorf("StepSummary(toast) = %q, want %q", got, want)
	}
}

func TestStepSummaryToastDefaultTitle(t *testing.T) {
	s := config.Step{Type: "toast", Message: "Done"}
	vars := tmpl.Vars{Profile: "boss"}
	got := StepSummary(s, &vars)
	want := `title="boss"  message="Done"`
	if got != want {
		t.Errorf("StepSummary(toast default title) = %q, want %q", got, want)
	}
}

func TestStepSummaryDiscord(t *testing.T) {
	s := config.Step{Type: "discord", Text: "{Profile} is ready"}
	vars := tmpl.Vars{Profile: "boss"}
	got := StepSummary(s, &vars)
	want := `text="Boss is ready"`
	if got != want {
		t.Errorf("StepSummary(discord) = %q, want %q", got, want)
	}
}

func TestStepSummaryDiscordVoice(t *testing.T) {
	s := config.Step{Type: "discord_voice", Text: "Done"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	want := `text="Done"`
	if got != want {
		t.Errorf("StepSummary(discord_voice) = %q, want %q", got, want)
	}
}

func TestStepSummarySlack(t *testing.T) {
	s := config.Step{Type: "slack", Text: "{Profile} is ready"}
	vars := tmpl.Vars{Profile: "boss"}
	got := StepSummary(s, &vars)
	want := `text="Boss is ready"`
	if got != want {
		t.Errorf("StepSummary(slack) = %q, want %q", got, want)
	}
}

func TestStepSummaryTelegram(t *testing.T) {
	s := config.Step{Type: "telegram", Text: "Done"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	want := `text="Done"`
	if got != want {
		t.Errorf("StepSummary(telegram) = %q, want %q", got, want)
	}
}

func TestStepSummaryTelegramAudio(t *testing.T) {
	s := config.Step{Type: "telegram_audio", Text: "Done"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	want := `text="Done"`
	if got != want {
		t.Errorf("StepSummary(telegram_audio) = %q, want %q", got, want)
	}
}

func TestStepSummaryTelegramVoice(t *testing.T) {
	s := config.Step{Type: "telegram_voice", Text: "Done"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	want := `text="Done"`
	if got != want {
		t.Errorf("StepSummary(telegram_voice) = %q, want %q", got, want)
	}
}

func TestStepSummaryUnknown(t *testing.T) {
	s := config.Step{Type: "bogus"}
	vars := tmpl.Vars{}
	got := StepSummary(s, &vars)
	if got != "" {
		t.Errorf("StepSummary(unknown) = %q, want empty", got)
	}
}

func TestStepSummaryNilVars(t *testing.T) {
	s := config.Step{Type: "say", Text: "{profile} ready"}
	got := StepSummary(s, nil)
	want := `text="{profile} ready"`
	if got != want {
		t.Errorf("StepSummary(nil vars) = %q, want %q", got, want)
	}
}

func TestStepSummaryWhenAndVolume(t *testing.T) {
	vol := 50
	s := config.Step{Type: "sound", Sound: "blip", When: "afk", Volume: &vol}
	got := StepSummary(s, nil)
	want := "sound=blip  when=afk  volume=50"
	if got != want {
		t.Errorf("StepSummary(when+volume) = %q, want %q", got, want)
	}
}
