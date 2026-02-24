package eventlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/paths"
	"github.com/Mavwarf/notify/internal/tmpl"
)

// LogSilentEnable appends a single line noting that silent mode was enabled.
// Best-effort, same as Log.
func LogSilentEnable(d time.Duration) {
	writeLog(func(f *os.File, ts string) {
		fmt.Fprintf(f, "%s  silent=enabled (%s)\n\n", ts, d)
	})
}

// LogSilentDisable appends a single line noting that silent mode was disabled.
// Best-effort, same as Log.
func LogSilentDisable() {
	writeLog(func(f *os.File, ts string) {
		fmt.Fprintf(f, "%s  silent=disabled\n\n", ts)
	})
}

// LogSilent appends a single line noting that an invocation was skipped
// due to silent mode. Best-effort, same as Log.
func LogSilent(profile, action string) {
	writeLog(func(f *os.File, ts string) {
		fmt.Fprintf(f, "%s  profile=%s  action=%s  silent=skipped\n\n",
			ts, profile, action)
	})
}

// LogCooldown appends a single line noting that an invocation was skipped
// due to cooldown. Best-effort, same as Log.
func LogCooldown(profile, action string, cooldownSeconds int) {
	writeLog(func(f *os.File, ts string) {
		fmt.Fprintf(f, "%s  profile=%s  action=%s  cooldown=skipped (%ds)\n\n",
			ts, profile, action, cooldownSeconds)
	})
}

// LogCooldownRecord appends a single line noting that cooldown state was
// updated for an action. Best-effort, same as Log.
func LogCooldownRecord(profile, action string, cooldownSeconds int) {
	writeLog(func(f *os.File, ts string) {
		fmt.Fprintf(f, "%s  profile=%s  action=%s  cooldown=recorded (%ds)\n",
			ts, profile, action, cooldownSeconds)
	})
}

// Log appends to the log file a summary line followed by one detail line
// per step. Errors are printed to stderr but never returned — logging is
// best-effort.
func Log(action string, steps []config.Step, afk bool, vars tmpl.Vars) {
	writeLog(func(f *os.File, ts string) {
		types := make([]string, len(steps))
		for i, s := range steps {
			types[i] = s.Type
		}

		// Summary line.
		fmt.Fprintf(f, "%s  profile=%s  action=%s  steps=%s  afk=%t\n",
			ts, vars.Profile, action, strings.Join(types, ","), afk)

		// Detail line per step.
		for i, s := range steps {
			detail := StepSummary(s, &vars)
			fmt.Fprintf(f, "%s    step[%d] %s  %s\n", ts, i+1, s.Type, detail)
		}

		// Blank line separates invocations.
		fmt.Fprintln(f)
	})
}

// writeLog opens the log file, generates a timestamp, and calls fn to
// write the entry. Errors are printed to stderr — logging is best-effort.
func writeLog(fn func(f *os.File, ts string)) {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()
	fn(f, time.Now().Format(time.RFC3339))
}

// openLog opens (or creates) the log file for appending, creating the
// parent directory if needed.
func openLog() (*os.File, error) {
	path := LogPath()
	if err := os.MkdirAll(filepath.Dir(path), paths.DirPerm); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, paths.FilePerm)
}

// LogPath returns the log file location:
//   - Windows: %APPDATA%\notify\notify.log
//   - Unix:    ~/.config/notify/notify.log
func LogPath() string {
	return filepath.Join(paths.DataDir(), paths.LogFileName)
}

// StepSummary returns a human-readable description of what a step does.
// When vars is non-nil, template variables are expanded (logging mode).
// When vars is nil, raw values are shown (dry-run mode).
// When/volume suffixes are always appended if present.
func StepSummary(s config.Step, vars *tmpl.Vars) string {
	// expand returns the expanded string if vars is set, otherwise the raw value.
	expand := func(text string) string {
		if vars != nil {
			return tmpl.Expand(text, *vars)
		}
		return text
	}

	var parts []string
	switch s.Type {
	case "sound":
		parts = append(parts, fmt.Sprintf("sound=%s", s.Sound))
	case "say":
		parts = append(parts, fmt.Sprintf("text=%q", expand(s.Text)))
	case "toast":
		title := s.Title
		if title == "" && vars != nil {
			title = vars.Profile
		}
		if title != "" {
			parts = append(parts, fmt.Sprintf("title=%q", expand(title)))
		}
		parts = append(parts, fmt.Sprintf("message=%q", expand(s.Message)))
	case "discord", "discord_voice", "slack", "telegram", "telegram_audio", "telegram_voice":
		parts = append(parts, fmt.Sprintf("text=%q", expand(s.Text)))
	case "webhook":
		parts = append(parts, fmt.Sprintf("url=%s", s.URL))
		parts = append(parts, fmt.Sprintf("text=%q", expand(s.Text)))
	}
	if s.When != "" {
		parts = append(parts, fmt.Sprintf("when=%s", s.When))
	}
	if s.Volume != nil {
		parts = append(parts, fmt.Sprintf("volume=%d", *s.Volume))
	}
	return strings.Join(parts, "  ")
}
