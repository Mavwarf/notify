package silent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mavwarf/notify/internal/paths"
)

type state struct {
	SilentUntil string `json:"silent_until"`
}

// IsSilent returns true if silent mode is currently active.
// A missing, unreadable, or corrupt state file is treated as "not silent"
// (fail-open).
func IsSilent() bool {
	return isSilent(statePath())
}

// SilentUntil returns the end time of silent mode and true if active,
// or zero time and false if not silent.
func SilentUntil() (time.Time, bool) {
	return silentUntil(statePath())
}

// Enable activates silent mode for the given duration from now.
func Enable(d time.Duration) {
	enable(statePath(), d)
}

// Disable deactivates silent mode by removing the state file.
func Disable() {
	disable(statePath())
}

func isSilent(path string) bool {
	_, ok := silentUntil(path)
	return ok
}

func silentUntil(path string) (time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return time.Time{}, false
	}

	t, err := time.Parse(time.RFC3339, s.SilentUntil)
	if err != nil {
		return time.Time{}, false
	}

	if time.Now().After(t) {
		return time.Time{}, false
	}

	return t, true
}

func enable(path string, d time.Duration) {
	s := state{SilentUntil: time.Now().Add(d).Format(time.RFC3339)}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "silent: marshal: %v\n", err)
		return
	}
	if err := paths.AtomicWrite(path, data); err != nil {
		fmt.Fprintf(os.Stderr, "silent: write: %v\n", err)
	}
}

func disable(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "silent: remove %s: %v\n", path, err)
	}
}

func statePath() string {
	return filepath.Join(paths.DataDir(), paths.SilentFileName)
}
