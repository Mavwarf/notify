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
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "%s  silent=enabled (%s)\n\n", ts, d)
}

// LogSilentDisable appends a single line noting that silent mode was disabled.
// Best-effort, same as Log.
func LogSilentDisable() {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "%s  silent=disabled\n\n", ts)
}

// LogSilent appends a single line noting that an invocation was skipped
// due to silent mode. Best-effort, same as Log.
func LogSilent(profile, action string) {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "%s  profile=%s  action=%s  silent=skipped\n\n",
		ts, profile, action)
}

// LogCooldown appends a single line noting that an invocation was skipped
// due to cooldown. Best-effort, same as Log.
func LogCooldown(profile, action string, cooldownSeconds int) {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "%s  profile=%s  action=%s  cooldown=skipped (%ds)\n\n",
		ts, profile, action, cooldownSeconds)
}

// LogCooldownRecord appends a single line noting that cooldown state was
// updated for an action. Best-effort, same as Log.
func LogCooldownRecord(profile, action string, cooldownSeconds int) {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "%s  profile=%s  action=%s  cooldown=recorded (%ds)\n",
		ts, profile, action, cooldownSeconds)
}

// Log appends to the log file a summary line followed by one detail line
// per step. Errors are printed to stderr but never returned — logging is
// best-effort.
func Log(action string, steps []config.Step, afk bool, vars tmpl.Vars) {
	f, err := openLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)

	types := make([]string, len(steps))
	for i, s := range steps {
		types[i] = s.Type
	}

	// Summary line.
	fmt.Fprintf(f, "%s  profile=%s  action=%s  steps=%s  afk=%t\n",
		ts, vars.Profile, action, strings.Join(types, ","), afk)

	// Detail line per step.
	for i, s := range steps {
		detail := stepDetail(s, vars)
		fmt.Fprintf(f, "%s    step[%d] %s  %s\n", ts, i+1, s.Type, detail)
	}

	// Blank line separates invocations.
	fmt.Fprintln(f)
}

// openLog opens (or creates) the log file for appending, creating the
// parent directory if needed.
func openLog() (*os.File, error) {
	path := logPath()
	if err := os.MkdirAll(filepath.Dir(path), paths.DirPerm); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, paths.FilePerm)
}

// logPath returns the log file location:
//   - Windows: %APPDATA%\notify\notify.log
//   - Unix:    ~/.config/notify/notify.log
func logPath() string {
	return filepath.Join(paths.DataDir(), paths.LogFileName)
}

// stepDetail returns a human-readable description of what a step does.
func stepDetail(s config.Step, vars tmpl.Vars) string {
	switch s.Type {
	case "sound":
		return fmt.Sprintf("sound=%s", s.Sound)
	case "say":
		return fmt.Sprintf("text=%q", tmpl.Expand(s.Text, vars))
	case "toast":
		title := s.Title
		if title == "" {
			title = vars.Profile
		}
		return fmt.Sprintf("title=%q message=%q", tmpl.Expand(title, vars), tmpl.Expand(s.Message, vars))
	case "discord", "discord_voice", "slack", "telegram", "telegram_audio", "telegram_voice":
		return fmt.Sprintf("text=%q", tmpl.Expand(s.Text, vars))
	default:
		return ""
	}
}
