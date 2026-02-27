package eventlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/paths"
	"github.com/Mavwarf/notify/internal/tmpl"
)

// Default is the package-level Store used by the convenience functions below.
// Set by OpenDefault() at program startup, or overridden in tests.
var Default Store

// OpenDefault initializes Default with the configured storage backend.
// Pass "file" for the flat-file backend, or "" / "sqlite" for SQLite.
// If SQLite fails to open, falls back to FileStore with a warning.
func OpenDefault(storage string) {
	switch storage {
	case "file":
		Default = NewFileStore(LogPath())
	default: // "" or "sqlite"
		dbPath := filepath.Join(paths.DataDir(), "notify.db")
		store, err := NewSQLiteStore(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "eventlog: sqlite: %v, falling back to file\n", err)
			Default = NewFileStore(LogPath())
			return
		}
		Default = store
	}
}

// Close closes the Default store if it implements io.Closer.
func Close() {
	if c, ok := Default.(io.Closer); ok {
		c.Close()
	}
}

// --- Write wrappers (best-effort, matching existing behavior) ---

// Log appends to the log file a summary line followed by one detail line
// per step. Errors are printed to stderr but never returned — logging is
// best-effort.
func Log(action string, steps []config.Step, afk bool, vars tmpl.Vars, desktop *int) {
	if err := Default.Log(action, steps, afk, vars, desktop); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// LogCooldown appends a single line noting that an invocation was skipped
// due to cooldown. Best-effort, same as Log.
func LogCooldown(profile, action string, cooldownSeconds int) {
	if err := Default.LogCooldown(profile, action, cooldownSeconds); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// LogCooldownRecord appends a single line noting that cooldown state was
// updated for an action. Best-effort, same as Log.
func LogCooldownRecord(profile, action string, cooldownSeconds int) {
	if err := Default.LogCooldownRecord(profile, action, cooldownSeconds); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// LogSilent appends a single line noting that an invocation was skipped
// due to silent mode. Best-effort, same as Log.
func LogSilent(profile, action string) {
	if err := Default.LogSilent(profile, action); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// LogSilentEnable appends a single line noting that silent mode was enabled.
// Best-effort, same as Log.
func LogSilentEnable(d time.Duration) {
	if err := Default.LogSilentEnable(d); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// LogSilentDisable appends a single line noting that silent mode was disabled.
// Best-effort, same as Log.
func LogSilentDisable() {
	if err := Default.LogSilentDisable(); err != nil {
		fmt.Fprintf(os.Stderr, "eventlog: %v\n", err)
	}
}

// --- Read wrappers ---

// Entries returns parsed log entries filtered to the last N days (0 = all).
func Entries(days int) ([]Entry, error) { return Default.Entries(days) }

// EntriesSince returns parsed log entries after the given cutoff time.
func EntriesSince(cutoff time.Time) ([]Entry, error) { return Default.EntriesSince(cutoff) }

// VoiceLines returns TTS text frequency data, optionally filtered to the last N days (0 = all).
func VoiceLines(days int) ([]VoiceLine, error) { return Default.VoiceLines(days) }

// ReadContent returns the raw log file content.
func ReadContent() (string, error) { return Default.ReadContent() }

// LogPath returns the log file location:
//   - Windows: %APPDATA%\notify\notify.log
//   - Unix:    ~/.config/notify/notify.log
//
// This is a variable so tests can override it.
var LogPath = func() string {
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
	case "plugin":
		parts = append(parts, fmt.Sprintf("command=%q", s.Command))
		if s.Text != "" {
			parts = append(parts, fmt.Sprintf("text=%q", expand(s.Text)))
		}
		if s.Timeout != nil {
			parts = append(parts, fmt.Sprintf("timeout=%d", *s.Timeout))
		}
	case "mqtt":
		parts = append(parts, fmt.Sprintf("broker=%s", s.Broker))
		parts = append(parts, fmt.Sprintf("topic=%s", s.Topic))
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
