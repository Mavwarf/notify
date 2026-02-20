package runner

import (
	"fmt"
	"sync"

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/discord"
	"github.com/Mavwarf/notify/internal/speech"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/toast"
)

// sequential returns true for step types that use the audio pipeline
// and must run one after another.
func sequential(typ string) bool {
	return typ == "sound" || typ == "say"
}

// FilterSteps returns only the steps that should run given the current
// AFK state. Steps with When="" always run; "afk" runs only when afk
// is true; "present" runs only when afk is false.
func FilterSteps(steps []config.Step, afk bool) []config.Step {
	out := make([]config.Step, 0, len(steps))
	for _, s := range steps {
		switch s.When {
		case "afk":
			if afk {
				out = append(out, s)
			}
		case "present":
			if !afk {
				out = append(out, s)
			}
		default:
			out = append(out, s)
		}
	}
	return out
}

// Execute runs the steps in the given action.
// Steps that don't use the audio pipeline (toast, etc.) are fired in
// parallel at the start. Audio-pipeline steps (sound, say) run
// sequentially in order.
func Execute(action *config.Action, defaultVolume int, profile, webhookURL string, afk bool) error {
	steps := FilterSteps(action.Steps, afk)

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
			if err := execStep(s, defaultVolume, profile, webhookURL); err != nil {
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
		if err := execStep(step, defaultVolume, profile, webhookURL); err != nil {
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

func execStep(step config.Step, defaultVolume int, profile, webhookURL string) error {
	vol := defaultVolume
	if step.Volume != nil {
		vol = *step.Volume
	}

	switch step.Type {
	case "sound":
		return audio.Play(step.Sound, float64(vol)/100.0)
	case "say":
		return speech.Say(tmpl.Expand(step.Text, profile), vol)
	case "toast":
		title := step.Title
		if title == "" {
			title = profile
		}
		return toast.Show(tmpl.Expand(title, profile), tmpl.Expand(step.Message, profile))
	case "discord":
		if webhookURL == "" {
			return fmt.Errorf("discord step requires credentials.discord_webhook in config")
		}
		return discord.Send(webhookURL, tmpl.Expand(step.Text, profile))
	default:
		return fmt.Errorf("unknown step type: %q", step.Type)
	}
}
