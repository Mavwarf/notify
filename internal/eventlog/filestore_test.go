package eventlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

// Compile-time interface check.
var _ Store = (*FileStore)(nil)

func tempStore(t *testing.T) *FileStore {
	t.Helper()
	return NewFileStore(filepath.Join(t.TempDir(), "notify.log"))
}

func TestFileStoreLogAndEntries(t *testing.T) {
	s := tempStore(t)
	vars := tmpl.Vars{Profile: "test"}
	steps := []config.Step{{Type: "sound", Sound: "blip"}, {Type: "say", Text: "hello"}}

	if err := s.Log("ready", steps, false, vars, nil); err != nil {
		t.Fatal(err)
	}

	entries, err := s.Entries(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Profile != "test" || entries[0].Action != "ready" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if entries[0].Kind != KindExecution {
		t.Fatalf("expected KindExecution, got %d", entries[0].Kind)
	}
}

func TestFileStoreLogCooldown(t *testing.T) {
	s := tempStore(t)

	if err := s.LogCooldown("p", "a", 30); err != nil {
		t.Fatal(err)
	}

	entries, _ := s.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != KindCooldown {
		t.Fatalf("expected KindCooldown, got %d", entries[0].Kind)
	}
}

func TestFileStoreLogSilent(t *testing.T) {
	s := tempStore(t)

	if err := s.LogSilent("p", "a"); err != nil {
		t.Fatal(err)
	}

	entries, _ := s.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != KindSilent {
		t.Fatalf("expected KindSilent, got %d", entries[0].Kind)
	}
}

func TestFileStoreLogSilentEnableDisable(t *testing.T) {
	s := tempStore(t)

	if err := s.LogSilentEnable(5 * time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.LogSilentDisable(); err != nil {
		t.Fatal(err)
	}

	content, err := s.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "silent=enabled") {
		t.Fatal("expected silent=enabled in log")
	}
	if !strings.Contains(content, "silent=disabled") {
		t.Fatal("expected silent=disabled in log")
	}
}

func TestFileStoreLogDesktop(t *testing.T) {
	s := tempStore(t)
	d := 2
	vars := tmpl.Vars{Profile: "test"}
	steps := []config.Step{{Type: "sound", Sound: "blip"}}

	if err := s.Log("ready", steps, false, vars, &d); err != nil {
		t.Fatal(err)
	}

	content, _ := s.ReadContent()
	if !strings.Contains(content, "desktop=2") {
		t.Fatal("expected desktop=2 in log")
	}
}

func TestFileStoreEntriesFilterByDays(t *testing.T) {
	s := tempStore(t)

	// Write entries directly to control timestamps.
	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -10).Format(time.RFC3339)

	content := today + "  profile=p  action=a  steps=sound  afk=false\n\n" +
		old + "  profile=p  action=old  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	all, _ := s.Entries(0)
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	recent, _ := s.Entries(3)
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent entry, got %d", len(recent))
	}
	if recent[0].Action != "a" {
		t.Fatalf("expected action 'a', got %q", recent[0].Action)
	}
}

func TestFileStoreEntriesSince(t *testing.T) {
	s := tempStore(t)

	now := time.Now()
	ts1 := now.Add(-2 * time.Hour).Format(time.RFC3339)
	ts2 := now.Add(-30 * time.Minute).Format(time.RFC3339)

	content := ts1 + "  profile=p  action=old  steps=sound  afk=false\n\n" +
		ts2 + "  profile=p  action=new  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	cutoff := now.Add(-1 * time.Hour)
	entries, err := s.EntriesSince(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry since cutoff, got %d", len(entries))
	}
	if entries[0].Action != "new" {
		t.Fatalf("expected action 'new', got %q", entries[0].Action)
	}
}

func TestFileStoreVoiceLines(t *testing.T) {
	s := tempStore(t)

	now := time.Now()
	ts := now.Format(time.RFC3339)

	content := ts + "  profile=p  action=a  steps=say  afk=false\n" +
		ts + "    step[1] say  text=\"hello world\"\n\n" +
		ts + "  profile=p  action=a  steps=say  afk=false\n" +
		ts + "    step[1] say  text=\"hello world\"\n\n" +
		ts + "  profile=p  action=a  steps=say  afk=false\n" +
		ts + "    step[1] say  text=\"goodbye\"\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	lines, err := s.VoiceLines(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 voice lines, got %d", len(lines))
	}
	// Sorted by frequency: "hello world" (2) first.
	if lines[0].Text != "hello world" || lines[0].Count != 2 {
		t.Fatalf("expected 'hello world' count 2, got %q count %d", lines[0].Text, lines[0].Count)
	}
}

func TestFileStoreVoiceLinesDaysFilter(t *testing.T) {
	s := tempStore(t)

	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -30).Format(time.RFC3339)

	content := today + "  profile=p  action=a  steps=say  afk=false\n" +
		today + "    step[1] say  text=\"recent\"\n\n" +
		old + "  profile=p  action=a  steps=say  afk=false\n" +
		old + "    step[1] say  text=\"ancient\"\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	lines, _ := s.VoiceLines(7)
	if len(lines) != 1 {
		t.Fatalf("expected 1 voice line in last 7 days, got %d", len(lines))
	}
	if lines[0].Text != "recent" {
		t.Fatalf("expected 'recent', got %q", lines[0].Text)
	}
}

func TestFileStoreReadContentEmpty(t *testing.T) {
	s := tempStore(t)

	content, err := s.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Fatalf("expected empty content for non-existent file, got %q", content)
	}
}

func TestFileStoreClear(t *testing.T) {
	s := tempStore(t)
	os.WriteFile(s.path, []byte("data"), 0644)

	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed after Clear")
	}
}

func TestFileStoreClearNonExistent(t *testing.T) {
	s := tempStore(t)
	// Should not error on non-existent file.
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear on non-existent file should not error: %v", err)
	}
}

func TestFileStoreClean(t *testing.T) {
	s := tempStore(t)

	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -30).Format(time.RFC3339)

	content := today + "  profile=p  action=new  steps=sound  afk=false\n\n" +
		old + "  profile=p  action=old  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	removed, err := s.Clean(7)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	// Remaining content should only have today's entry.
	remaining, _ := s.ReadContent()
	if strings.Contains(remaining, "action=old") {
		t.Fatal("old entry should have been cleaned")
	}
	if !strings.Contains(remaining, "action=new") {
		t.Fatal("new entry should remain")
	}
}

func TestFileStoreCleanEmpty(t *testing.T) {
	s := tempStore(t)

	removed, err := s.Clean(7)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed for non-existent file, got %d", removed)
	}
}

func TestFileStoreRemoveProfile(t *testing.T) {
	s := tempStore(t)

	now := time.Now().Format(time.RFC3339)
	content := now + "  profile=keep  action=a  steps=sound  afk=false\n\n" +
		now + "  profile=remove  action=a  steps=sound  afk=false\n\n" +
		now + "  profile=remove  action=b  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	removed, err := s.RemoveProfile("remove")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	remaining, _ := s.ReadContent()
	if strings.Contains(remaining, "profile=remove") {
		t.Fatal("removed profile entries should be gone")
	}
	if !strings.Contains(remaining, "profile=keep") {
		t.Fatal("kept profile entry should remain")
	}
}

func TestFileStoreRemoveProfileNotFound(t *testing.T) {
	s := tempStore(t)

	now := time.Now().Format(time.RFC3339)
	content := now + "  profile=keep  action=a  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	removed, err := s.RemoveProfile("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestFileStoreEntriesNonExistent(t *testing.T) {
	s := tempStore(t)

	entries, err := s.Entries(0)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for non-existent file, got %v", entries)
	}
}

func TestFileStoreEntriesSinceNonExistent(t *testing.T) {
	s := tempStore(t)

	entries, err := s.EntriesSince(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for non-existent file, got %v", entries)
	}
}

func TestFileStoreVoiceLinesNonExistent(t *testing.T) {
	s := tempStore(t)

	lines, err := s.VoiceLines(0)
	if err != nil {
		t.Fatal(err)
	}
	if lines != nil {
		t.Fatalf("expected nil lines for non-existent file, got %v", lines)
	}
}

func TestFileStorePath(t *testing.T) {
	path := "/some/path/notify.log"
	s := NewFileStore(path)
	if s.Path() != path {
		t.Fatalf("expected path %q, got %q", path, s.Path())
	}
}

func TestFileStoreLogCooldownRecord(t *testing.T) {
	s := tempStore(t)

	if err := s.LogCooldownRecord("p", "a", 60); err != nil {
		t.Fatal(err)
	}

	content, _ := s.ReadContent()
	if !strings.Contains(content, "cooldown=recorded (60s)") {
		t.Fatal("expected cooldown=recorded in log")
	}
}

func TestFileStoreCleanRemovesAll(t *testing.T) {
	s := tempStore(t)

	old := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	content := old + "  profile=p  action=a  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	removed, err := s.Clean(7)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	// File should be removed when all entries are cleaned.
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed when all entries cleaned")
	}
}

func TestFileStoreRemoveProfileClearsFile(t *testing.T) {
	s := tempStore(t)

	now := time.Now().Format(time.RFC3339)
	content := now + "  profile=only  action=a  steps=sound  afk=false\n\n"
	os.WriteFile(s.path, []byte(content), 0644)

	removed, _ := s.RemoveProfile("only")
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	// File should be removed when all entries for the only profile are gone.
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
}
