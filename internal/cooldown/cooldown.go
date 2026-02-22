package cooldown

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mavwarf/notify/internal/paths"
)

// Check returns true if the given profile/action is still within its cooldown
// window. A missing or unreadable state file is treated as "not on cooldown"
// (fail-open).
func Check(profile, action string, cooldownSeconds int) bool {
	return check(statePath(), profile, action, cooldownSeconds)
}

// Record writes the current timestamp for the given profile/action.
// Errors are printed to stderr but never fatal (best-effort).
func Record(profile, action string) {
	record(statePath(), profile, action)
}

func check(path, profile, action string, cooldownSeconds int) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false // missing or unreadable → allow
	}

	var state map[string]string
	if err := json.Unmarshal(data, &state); err != nil {
		return false // corrupt → allow
	}

	key := paths.CooldownKey(profile, action)
	ts, ok := state[key]
	if !ok {
		return false
	}

	last, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}

	return time.Since(last) < time.Duration(cooldownSeconds)*time.Second
}

func record(path, profile, action string) {
	// Ensure directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, paths.DirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "cooldown: mkdir %s: %v\n", dir, err)
		return
	}

	// Load existing state.
	state := make(map[string]string)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &state) // ignore corrupt; overwrite
	}

	// Prune expired entries.
	for k, v := range state {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil || time.Since(t) > 24*time.Hour {
			delete(state, k)
		}
	}

	key := paths.CooldownKey(profile, action)
	state[key] = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cooldown: marshal: %v\n", err)
		return
	}

	// Atomic write: tmp file then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, paths.FilePerm); err != nil {
		fmt.Fprintf(os.Stderr, "cooldown: write %s: %v\n", tmp, err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		fmt.Fprintf(os.Stderr, "cooldown: rename %s → %s: %v\n", tmp, path, err)
	}
}

func statePath() string {
	return filepath.Join(paths.DataDir(), paths.CooldownFileName)
}
