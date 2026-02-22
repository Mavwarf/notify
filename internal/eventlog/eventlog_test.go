package eventlog

import (
	"testing"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

func TestStepDetailSound(t *testing.T) {
	s := config.Step{Type: "sound", Sound: "blip"}
	got := stepDetail(s, tmpl.Vars{})
	want := "sound=blip"
	if got != want {
		t.Errorf("stepDetail(sound) = %q, want %q", got, want)
	}
}

func TestStepDetailSay(t *testing.T) {
	s := config.Step{Type: "say", Text: "{profile} ready"}
	got := stepDetail(s, tmpl.Vars{Profile: "boss"})
	want := `text="boss ready"`
	if got != want {
		t.Errorf("stepDetail(say) = %q, want %q", got, want)
	}
}

func TestStepDetailToast(t *testing.T) {
	s := config.Step{Type: "toast", Title: "Build", Message: "{Profile} done"}
	got := stepDetail(s, tmpl.Vars{Profile: "boss"})
	want := `title="Build" message="Boss done"`
	if got != want {
		t.Errorf("stepDetail(toast) = %q, want %q", got, want)
	}
}

func TestStepDetailToastDefaultTitle(t *testing.T) {
	s := config.Step{Type: "toast", Message: "Done"}
	got := stepDetail(s, tmpl.Vars{Profile: "boss"})
	want := `title="boss" message="Done"`
	if got != want {
		t.Errorf("stepDetail(toast default title) = %q, want %q", got, want)
	}
}

func TestStepDetailDiscord(t *testing.T) {
	s := config.Step{Type: "discord", Text: "{Profile} is ready"}
	got := stepDetail(s, tmpl.Vars{Profile: "boss"})
	want := `text="Boss is ready"`
	if got != want {
		t.Errorf("stepDetail(discord) = %q, want %q", got, want)
	}
}

func TestStepDetailDiscordVoice(t *testing.T) {
	s := config.Step{Type: "discord_voice", Text: "Done"}
	got := stepDetail(s, tmpl.Vars{})
	want := `text="Done"`
	if got != want {
		t.Errorf("stepDetail(discord_voice) = %q, want %q", got, want)
	}
}

func TestStepDetailSlack(t *testing.T) {
	s := config.Step{Type: "slack", Text: "{Profile} is ready"}
	got := stepDetail(s, tmpl.Vars{Profile: "boss"})
	want := `text="Boss is ready"`
	if got != want {
		t.Errorf("stepDetail(slack) = %q, want %q", got, want)
	}
}

func TestStepDetailTelegram(t *testing.T) {
	s := config.Step{Type: "telegram", Text: "Done"}
	got := stepDetail(s, tmpl.Vars{})
	want := `text="Done"`
	if got != want {
		t.Errorf("stepDetail(telegram) = %q, want %q", got, want)
	}
}

func TestStepDetailTelegramAudio(t *testing.T) {
	s := config.Step{Type: "telegram_audio", Text: "Done"}
	got := stepDetail(s, tmpl.Vars{})
	want := `text="Done"`
	if got != want {
		t.Errorf("stepDetail(telegram_audio) = %q, want %q", got, want)
	}
}

func TestStepDetailTelegramVoice(t *testing.T) {
	s := config.Step{Type: "telegram_voice", Text: "Done"}
	got := stepDetail(s, tmpl.Vars{})
	want := `text="Done"`
	if got != want {
		t.Errorf("stepDetail(telegram_voice) = %q, want %q", got, want)
	}
}

func TestStepDetailUnknown(t *testing.T) {
	s := config.Step{Type: "bogus"}
	got := stepDetail(s, tmpl.Vars{})
	if got != "" {
		t.Errorf("stepDetail(unknown) = %q, want empty", got)
	}
}
