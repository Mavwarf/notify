package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/discord"
	"github.com/Mavwarf/notify/internal/speech"
	"github.com/Mavwarf/notify/internal/telegram"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/toast"
)

// sequential returns true for step types that use the audio pipeline
// and must run one after another.
func sequential(typ string) bool {
	return typ == "sound" || typ == "say"
}

// FilterSteps returns only the steps that should run given the current
// AFK state and invocation mode. Steps with When="" always run;
// "afk"/"present" filter on idle state; "run"/"direct" filter on
// whether the invocation came from `notify run`; "hours:X-Y" filters
// on the current hour (24h local time).
func FilterSteps(steps []config.Step, afk, run bool) []config.Step {
	now := time.Now()
	out := make([]config.Step, 0, len(steps))
	for _, s := range steps {
		if matchWhen(s.When, afk, run, now) {
			out = append(out, s)
		}
	}
	return out
}

// matchWhen evaluates a single "when" condition string against the
// current state. Unknown conditions return false (fail-closed) with a
// warning printed to stderr.
func matchWhen(when string, afk, run bool, now time.Time) bool {
	switch when {
	case "":
		return true
	case "afk":
		return afk
	case "present":
		return !afk
	case "run":
		return run
	case "direct":
		return !run
	default:
		if strings.HasPrefix(when, "hours:") {
			return matchHours(when[6:], now)
		}
		fmt.Fprintf(os.Stderr, "warning: unknown when condition %q, skipping step\n", when)
		return false
	}
}

// matchHours parses a spec like "8-22" and returns true if now's hour
// falls within the range. Cross-midnight ranges ("22-8") are supported.
// Same start and end ("8-8") is a zero-width window and never matches.
// Returns false (skip step) on parse errors.
func matchHours(spec string, now time.Time) bool {
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "warning: invalid hours spec %q, skipping step\n", spec)
		return false
	}
	start, err1 := strconv.Atoi(parts[0])
	end, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || start < 0 || start > 23 || end < 0 || end > 23 {
		fmt.Fprintf(os.Stderr, "warning: invalid hours spec %q, skipping step\n", spec)
		return false
	}
	h := now.Hour()
	if start == end {
		return false
	}
	if start < end {
		return h >= start && h < end
	}
	// Cross-midnight: e.g. 22-8 → hour >= 22 OR hour < 8
	return h >= start || h < end
}

// Execute runs the steps in the given action.
// Steps that don't use the audio pipeline (toast, etc.) are fired in
// parallel at the start. Audio-pipeline steps (sound, say) run
// sequentially in order.
func Execute(action *config.Action, defaultVolume int, creds config.Credentials, vars tmpl.Vars, afk, run bool) error {
	steps := FilterSteps(action.Steps, afk, run)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var parallelErrs []error

	// Launch non-sequential steps in parallel immediately.
	for i, step := range steps {
		if sequential(step.Type) {
			continue
		}
		wg.Add(1)
		go func(idx int, s config.Step) {
			defer wg.Done()
			if err := execStep(s, defaultVolume, creds, vars); err != nil {
				mu.Lock()
				parallelErrs = append(parallelErrs, fmt.Errorf("step %d (%s): %w", idx+1, s.Type, err))
				mu.Unlock()
			}
		}(i, step)
	}

	// Run sequential (audio-pipeline) steps in order.
	for i, step := range steps {
		if !sequential(step.Type) {
			continue
		}
		if err := execStep(step, defaultVolume, creds, vars); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Type, err)
		}
	}

	// Wait for parallel steps to finish.
	wg.Wait()

	if len(parallelErrs) > 0 {
		return parallelErrs[0]
	}
	return nil
}

func execStep(step config.Step, defaultVolume int, creds config.Credentials, vars tmpl.Vars) error {
	vol := defaultVolume
	if step.Volume != nil {
		vol = *step.Volume
	}

	switch step.Type {
	case "sound":
		return audio.Play(step.Sound, float64(vol)/100.0)
	case "say":
		return speech.Say(tmpl.Expand(step.Text, vars), vol)
	case "toast":
		title := step.Title
		if title == "" {
			title = vars.Profile
		}
		return toast.Show(tmpl.Expand(title, vars), tmpl.Expand(step.Message, vars))
	case "discord":
		if creds.DiscordWebhook == "" {
			return fmt.Errorf("discord step requires credentials.discord_webhook in config")
		}
		return discord.Send(creds.DiscordWebhook, tmpl.Expand(step.Text, vars))
	case "discord_voice":
		if creds.DiscordWebhook == "" {
			return fmt.Errorf("discord_voice step requires credentials.discord_webhook in config")
		}
		text := tmpl.Expand(step.Text, vars)
		wavFile, err := os.CreateTemp("", "notify-voice-*.wav")
		if err != nil {
			return fmt.Errorf("discord_voice temp file: %w", err)
		}
		wavPath := wavFile.Name()
		wavFile.Close()
		if err := speech.SayToFile(text, vol, wavPath); err != nil {
			return fmt.Errorf("discord_voice tts: %w", err)
		}
		defer os.Remove(wavPath)
		return discord.SendVoice(creds.DiscordWebhook, wavPath, text)
	case "telegram":
		if creds.TelegramToken == "" || creds.TelegramChatID == "" {
			return fmt.Errorf("telegram step requires credentials.telegram_token and telegram_chat_id in config")
		}
		return telegram.Send(creds.TelegramToken, creds.TelegramChatID, tmpl.Expand(step.Text, vars))
	default:
		return fmt.Errorf("unknown step type: %q", step.Type)
	}
}
