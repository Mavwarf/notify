package eventlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

// Log appends to ~/.notify.log a summary line followed by one detail line
// per step. Errors are printed to stderr but never returned — logging is
// best-effort.
func Log(action string, steps []config.Step, afk bool, vars tmpl.Vars) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
		return
	}

	path := filepath.Join(home, ".notify.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	case "discord":
		return fmt.Sprintf("text=%q", tmpl.Expand(s.Text, vars))
	default:
		return ""
	}
}
