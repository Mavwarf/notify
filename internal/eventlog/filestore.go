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

// FileStore implements Store using a flat log file.
type FileStore struct {
	path string
}

// NewFileStore returns a FileStore that reads and writes the given log file.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// openLog opens (or creates) the log file for appending, creating the
// parent directory if needed.
func (f *FileStore) openLog() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(f.path), paths.DirPerm); err != nil {
		return nil, err
	}
	return os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, paths.FilePerm)
}

// writeLog opens the log file, generates a timestamp, and calls fn to
// write the entry.
func (f *FileStore) writeLog(fn func(file *os.File, ts string)) error {
	file, err := f.openLog()
	if err != nil {
		return err
	}
	defer file.Close()
	fn(file, time.Now().Format(time.RFC3339))
	return nil
}

func (f *FileStore) Log(action string, steps []config.Step, afk bool, vars tmpl.Vars, desktop *int) error {
	return f.writeLog(func(file *os.File, ts string) {
		types := make([]string, len(steps))
		for i, s := range steps {
			types[i] = s.Type
		}

		summary := fmt.Sprintf("%s  profile=%s  action=%s  steps=%s  afk=%t",
			ts, vars.Profile, action, strings.Join(types, ","), afk)
		if desktop != nil {
			summary += fmt.Sprintf("  desktop=%d", *desktop)
		}
		if vars.ClaudeHook != "" {
			summary += fmt.Sprintf("  claude_hook=%s", vars.ClaudeHook)
		}
		if vars.ClaudeMessage != "" {
			summary += fmt.Sprintf("  claude_message=%q", vars.ClaudeMessage)
		}
		fmt.Fprintln(file, summary)

		for i, s := range steps {
			detail := StepSummary(s, &vars)
			fmt.Fprintf(file, "%s    step[%d] %s  %s\n", ts, i+1, s.Type, detail)
		}

		fmt.Fprintln(file)
	})
}

func (f *FileStore) LogCooldown(profile, action string, seconds int) error {
	return f.writeLog(func(file *os.File, ts string) {
		fmt.Fprintf(file, "%s  profile=%s  action=%s  cooldown=skipped (%ds)\n\n",
			ts, profile, action, seconds)
	})
}

func (f *FileStore) LogCooldownRecord(profile, action string, seconds int) error {
	return f.writeLog(func(file *os.File, ts string) {
		fmt.Fprintf(file, "%s  profile=%s  action=%s  cooldown=recorded (%ds)\n",
			ts, profile, action, seconds)
	})
}

func (f *FileStore) LogSilent(profile, action string) error {
	return f.writeLog(func(file *os.File, ts string) {
		fmt.Fprintf(file, "%s  profile=%s  action=%s  silent=skipped\n\n",
			ts, profile, action)
	})
}

func (f *FileStore) LogSilentEnable(d time.Duration) error {
	return f.writeLog(func(file *os.File, ts string) {
		fmt.Fprintf(file, "%s  silent=enabled (%s)\n\n", ts, d)
	})
}

func (f *FileStore) LogSilentDisable() error {
	return f.writeLog(func(file *os.File, ts string) {
		fmt.Fprintf(file, "%s  silent=disabled\n\n", ts)
	})
}

func (f *FileStore) Entries(days int) ([]Entry, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries := ParseEntries(string(data))
	if days <= 0 {
		return entries, nil
	}

	cutoff := DayCutoff(days)
	var filtered []Entry
	for _, e := range entries {
		if !e.Time.In(cutoff.Location()).Before(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (f *FileStore) EntriesSince(cutoff time.Time) ([]Entry, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries := ParseEntries(string(data))
	var filtered []Entry
	for _, e := range entries {
		if !e.Time.Before(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (f *FileStore) VoiceLines(days int) ([]VoiceLine, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	content := string(data)
	if days > 0 {
		content = FilterBlocksByDays(content, days)
	}
	return ParseVoiceLines(content), nil
}

func (f *FileStore) ReadContent() (string, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (f *FileStore) Clean(days int) (int, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		return 0, nil
	}

	origBlocks := 0
	for _, b := range strings.Split(content, "\n\n") {
		if strings.TrimSpace(b) != "" {
			origBlocks++
		}
	}

	filtered := FilterBlocksByDays(content, days)

	keptBlocks := 0
	if filtered != "" {
		for _, b := range strings.Split(filtered, "\n\n") {
			if strings.TrimSpace(b) != "" {
				keptBlocks++
			}
		}
	}
	removed := origBlocks - keptBlocks

	if filtered == "" {
		_ = os.Remove(f.path)
		return removed, nil
	}

	out := filtered + "\n\n"
	if err := os.WriteFile(f.path, []byte(out), paths.FilePerm); err != nil {
		return 0, err
	}
	return removed, nil
}

func (f *FileStore) RemoveProfile(name string) (int, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		return 0, nil
	}

	filtered, removed := FilterBlocksByProfile(content, name)
	if removed == 0 {
		return 0, nil
	}

	if filtered == "" {
		_ = os.Remove(f.path)
		return removed, nil
	}

	out := filtered + "\n\n"
	if err := os.WriteFile(f.path, []byte(out), paths.FilePerm); err != nil {
		return 0, err
	}
	return removed, nil
}

func (f *FileStore) Clear() error {
	err := os.Remove(f.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (f *FileStore) Path() string {
	return f.path
}
