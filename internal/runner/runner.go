package runner

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/discord"
	"github.com/Mavwarf/notify/internal/ffmpeg"
	"github.com/Mavwarf/notify/internal/slack"
	"github.com/Mavwarf/notify/internal/speech"
	"github.com/Mavwarf/notify/internal/telegram"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/toast"
	"github.com/Mavwarf/notify/internal/webhook"
)

// remoteVolume is used for TTS in remote voice steps. Volume control
// is left to the receiving side, so we always render at full volume.
const remoteVolume = 100

// ttsToTempFile renders text to a temporary WAV file via TTS and returns the
// file path plus a cleanup function that removes the temp file.
func ttsToTempFile(prefix, text string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", nil, fmt.Errorf("temp file: %w", err)
	}
	path = f.Name()
	if err := f.Close(); err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	if err := speech.SayToFile(text, remoteVolume, path); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("tts: %w", err)
	}
	cleanup = func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

// retryOnce calls fn and, if it fails, waits 2 seconds and tries once
// more. Used for remote network calls so a single transient error
// doesn't lose the notification.
func retryOnce(fn func() error) error {
	if err := fn(); err != nil {
		time.Sleep(2 * time.Second)
		return fn()
	}
	return nil
}

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

// FilteredIndices returns a boolean map indicating which step indices
// would run. Used by dry-run to mark each step as RUN or SKIP.
func FilteredIndices(steps []config.Step, afk, run bool) map[int]bool {
	now := time.Now()
	m := make(map[int]bool, len(steps))
	for i, s := range steps {
		if matchWhen(s.When, afk, run, now) {
			m[i] = true
		}
	}
	return m
}

// matchWhen evaluates a single "when" condition string against the
// current state. Unknown conditions return false (fail-closed) with a
// warning printed to stderr.
func matchWhen(when string, afk, run bool, now time.Time) bool {
	switch when {
	case "":
		return true
	case "never":
		return false
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

// Execute runs the given steps (already filtered by the caller).
// Steps that don't use the audio pipeline (toast, etc.) are fired in
// parallel at the start. Audio-pipeline steps (sound, say) run
// sequentially in order.
func Execute(steps []config.Step, defaultVolume int, creds config.Credentials, vars tmpl.Vars) error {

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
			if err := stepExec(s, defaultVolume, creds, vars); err != nil {
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
		if err := stepExec(step, defaultVolume, creds, vars); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Type, err)
		}
	}

	// Wait for parallel steps to finish.
	wg.Wait()

	if len(parallelErrs) > 0 {
		return errors.Join(parallelErrs...)
	}
	return nil
}

// stepExec is the function used to execute a single step. It can be
// replaced in tests to avoid real audio/network calls.
var stepExec = execStep

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
		msg := tmpl.Expand(step.Text, vars)
		return retryOnce(func() error { return discord.Send(creds.DiscordWebhook, msg) })
	case "discord_voice":
		text := tmpl.Expand(step.Text, vars)
		wavPath, cleanup, err := ttsToTempFile("notify-voice-*.wav", text)
		if err != nil {
			return fmt.Errorf("discord_voice: %w", err)
		}
		defer cleanup()
		return retryOnce(func() error { return discord.SendVoice(creds.DiscordWebhook, wavPath, text) })
	case "slack":
		msg := tmpl.Expand(step.Text, vars)
		return retryOnce(func() error { return slack.Send(creds.SlackWebhook, msg) })
	case "telegram":
		msg := tmpl.Expand(step.Text, vars)
		return retryOnce(func() error { return telegram.Send(creds.TelegramToken, creds.TelegramChatID, msg) })
	case "telegram_audio":
		text := tmpl.Expand(step.Text, vars)
		wavPath, cleanup, err := ttsToTempFile("notify-tgaudio-*.wav", text)
		if err != nil {
			return fmt.Errorf("telegram_audio: %w", err)
		}
		defer cleanup()
		return retryOnce(func() error { return telegram.SendAudio(creds.TelegramToken, creds.TelegramChatID, wavPath, text) })
	case "telegram_voice":
		text := tmpl.Expand(step.Text, vars)
		wavPath, wavCleanup, err := ttsToTempFile("notify-tgvoice-*.wav", text)
		if err != nil {
			return fmt.Errorf("telegram_voice: %w", err)
		}
		defer wavCleanup()
		oggFile, err := os.CreateTemp("", "notify-tgvoice-*.ogg")
		if err != nil {
			return fmt.Errorf("telegram_voice ogg temp file: %w", err)
		}
		oggPath := oggFile.Name()
		if err := oggFile.Close(); err != nil {
			return fmt.Errorf("telegram_voice close ogg temp: %w", err)
		}
		if err := ffmpeg.ToOGG(wavPath, oggPath); err != nil {
			return fmt.Errorf("telegram_voice convert: %w", err)
		}
		defer func() { _ = os.Remove(oggPath) }()
		return retryOnce(func() error { return telegram.SendVoice(creds.TelegramToken, creds.TelegramChatID, oggPath, text) })
	case "webhook":
		msg := tmpl.Expand(step.Text, vars)
		return retryOnce(func() error { return webhook.Send(step.URL, msg, step.Headers) })
	default:
		return fmt.Errorf("unknown step type: %q", step.Type)
	}
}
