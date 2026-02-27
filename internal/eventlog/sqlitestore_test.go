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
var _ Store = (*SQLiteStore)(nil)

func tempSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notify.db")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStoreLogAndEntries(t *testing.T) {
	s := tempSQLiteStore(t)
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

func TestSQLiteStoreLogCooldown(t *testing.T) {
	s := tempSQLiteStore(t)

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

func TestSQLiteStoreLogSilent(t *testing.T) {
	s := tempSQLiteStore(t)

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

func TestSQLiteStoreLogSilentEnableDisable(t *testing.T) {
	s := tempSQLiteStore(t)

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

func TestSQLiteStoreLogDesktop(t *testing.T) {
	s := tempSQLiteStore(t)
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

func TestSQLiteStoreEntriesFilterByDays(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -10).Format(time.RFC3339)

	// Insert events directly.
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		today, "p", "a", int(KindExecution), "sound")
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		old, "p", "old", int(KindExecution), "sound")

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

func TestSQLiteStoreEntriesSince(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now()
	ts1 := now.Add(-2 * time.Hour).Format(time.RFC3339)
	ts2 := now.Add(-30 * time.Minute).Format(time.RFC3339)

	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		ts1, "p", "old", int(KindExecution), "sound")
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		ts2, "p", "new", int(KindExecution), "sound")

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

func TestSQLiteStoreVoiceLines(t *testing.T) {
	s := tempSQLiteStore(t)

	vars := tmpl.Vars{Profile: "p"}
	steps := []config.Step{{Type: "say", Text: "hello world"}}

	// Log three times: 2x "hello world", 1x "goodbye"
	s.Log("a", steps, false, vars, nil)
	s.Log("a", steps, false, vars, nil)
	steps2 := []config.Step{{Type: "say", Text: "goodbye"}}
	s.Log("a", steps2, false, vars, nil)

	lines, err := s.VoiceLines(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 voice lines, got %d", len(lines))
	}
	if lines[0].Text != "hello world" || lines[0].Count != 2 {
		t.Fatalf("expected 'hello world' count 2, got %q count %d", lines[0].Text, lines[0].Count)
	}
}

func TestSQLiteStoreVoiceLinesDaysFilter(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -30).Format(time.RFC3339)

	// Insert events with step_details directly for timestamp control.
	res1, _ := s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		today, "p", "a", int(KindExecution), "say")
	id1, _ := res1.LastInsertId()
	s.db.Exec(`INSERT INTO step_details (event_id, step_num, step_type, detail, voice_text) VALUES (?, ?, ?, ?, ?)`,
		id1, 1, "say", `text="recent"`, "recent")

	res2, _ := s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		old, "p", "a", int(KindExecution), "say")
	id2, _ := res2.LastInsertId()
	s.db.Exec(`INSERT INTO step_details (event_id, step_num, step_type, detail, voice_text) VALUES (?, ?, ?, ?, ?)`,
		id2, 1, "say", `text="ancient"`, "ancient")

	lines, _ := s.VoiceLines(7)
	if len(lines) != 1 {
		t.Fatalf("expected 1 voice line in last 7 days, got %d", len(lines))
	}
	if lines[0].Text != "recent" {
		t.Fatalf("expected 'recent', got %q", lines[0].Text)
	}
}

func TestSQLiteStoreReadContentEmpty(t *testing.T) {
	s := tempSQLiteStore(t)

	content, err := s.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Fatalf("expected empty content for empty DB, got %q", content)
	}
}

func TestSQLiteStoreClear(t *testing.T) {
	s := tempSQLiteStore(t)

	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind) VALUES (?, ?, ?, ?)`,
		time.Now().Format(time.RFC3339), "p", "a", int(KindExecution))

	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}

	entries, _ := s.Entries(0)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestSQLiteStoreClean(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now()
	today := now.Format(time.RFC3339)
	old := now.AddDate(0, 0, -30).Format(time.RFC3339)

	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		today, "p", "new", int(KindExecution), "sound")
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		old, "p", "old", int(KindExecution), "sound")

	removed, err := s.Clean(7)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	entries, _ := s.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 remaining entry, got %d", len(entries))
	}
	if entries[0].Action != "new" {
		t.Fatalf("expected action 'new', got %q", entries[0].Action)
	}
}

func TestSQLiteStoreCleanEmpty(t *testing.T) {
	s := tempSQLiteStore(t)

	removed, err := s.Clean(7)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed for empty DB, got %d", removed)
	}
}

func TestSQLiteStoreRemoveProfile(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now().Format(time.RFC3339)
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		now, "keep", "a", int(KindExecution), "sound")
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		now, "remove", "a", int(KindExecution), "sound")
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		now, "remove", "b", int(KindExecution), "sound")

	removed, err := s.RemoveProfile("remove")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	entries, _ := s.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 remaining entry, got %d", len(entries))
	}
	if entries[0].Profile != "keep" {
		t.Fatalf("expected profile 'keep', got %q", entries[0].Profile)
	}
}

func TestSQLiteStoreRemoveProfileNotFound(t *testing.T) {
	s := tempSQLiteStore(t)

	now := time.Now().Format(time.RFC3339)
	s.db.Exec(`INSERT INTO events (timestamp, profile, action, kind, steps_csv) VALUES (?, ?, ?, ?, ?)`,
		now, "keep", "a", int(KindExecution), "sound")

	removed, err := s.RemoveProfile("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestSQLiteStoreEntriesEmpty(t *testing.T) {
	s := tempSQLiteStore(t)

	entries, err := s.Entries(0)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for empty DB, got %v", entries)
	}
}

func TestSQLiteStoreEntriesSinceEmpty(t *testing.T) {
	s := tempSQLiteStore(t)

	entries, err := s.EntriesSince(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for empty DB, got %v", entries)
	}
}

func TestSQLiteStoreVoiceLinesEmpty(t *testing.T) {
	s := tempSQLiteStore(t)

	lines, err := s.VoiceLines(0)
	if err != nil {
		t.Fatal(err)
	}
	if lines != nil {
		t.Fatalf("expected nil lines for empty DB, got %v", lines)
	}
}

func TestSQLiteStorePath(t *testing.T) {
	s := tempSQLiteStore(t)
	if !strings.HasSuffix(s.Path(), "notify.db") {
		t.Fatalf("expected path ending in notify.db, got %q", s.Path())
	}
}

func TestSQLiteStoreLogCooldownRecord(t *testing.T) {
	s := tempSQLiteStore(t)

	if err := s.LogCooldownRecord("p", "a", 60); err != nil {
		t.Fatal(err)
	}

	content, _ := s.ReadContent()
	if !strings.Contains(content, "cooldown=recorded (60s)") {
		t.Fatal("expected cooldown=recorded in log")
	}
}

func TestSQLiteStoreCascadeDelete(t *testing.T) {
	s := tempSQLiteStore(t)

	vars := tmpl.Vars{Profile: "p"}
	steps := []config.Step{{Type: "say", Text: "hello"}}
	s.Log("a", steps, false, vars, nil)

	// Verify step_details exist.
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM step_details`).Scan(&count)
	if count == 0 {
		t.Fatal("expected step_details after Log")
	}

	// Clear should cascade delete step_details.
	s.Clear()
	s.db.QueryRow(`SELECT COUNT(*) FROM step_details`).Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 step_details after Clear, got %d", count)
	}
}

func TestSQLiteStoreSilentEnableDisableNotInEntries(t *testing.T) {
	s := tempSQLiteStore(t)

	s.LogSilentEnable(5 * time.Minute)
	s.LogSilentDisable()

	// Silent enable/disable have no profile/action, so Entries() should skip them.
	entries, _ := s.Entries(0)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (silent enable/disable filtered), got %d", len(entries))
	}
}

func TestSQLiteStoreMigration(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "notify.log")

	now := time.Now()
	ts := now.Format(time.RFC3339)

	content := ts + "  profile=test  action=ready  steps=sound,say  afk=false\n" +
		ts + "    step[1] sound  sound=blip\n" +
		ts + "    step[2] say  text=\"hello world\"\n\n" +
		ts + "  profile=test  action=ready  cooldown=skipped (30s)\n\n"
	os.WriteFile(logPath, []byte(content), 0644)

	dbPath := filepath.Join(dir, "notify.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Check entries were migrated.
	entries, _ := s.Entries(0)
	if len(entries) != 2 {
		t.Fatalf("expected 2 migrated entries, got %d", len(entries))
	}
	if entries[0].Kind != KindExecution {
		t.Fatalf("expected KindExecution, got %d", entries[0].Kind)
	}
	if entries[1].Kind != KindCooldown {
		t.Fatalf("expected KindCooldown, got %d", entries[1].Kind)
	}

	// Check voice lines were migrated.
	lines, _ := s.VoiceLines(0)
	if len(lines) != 1 || lines[0].Text != "hello world" {
		t.Fatalf("expected migrated voice line 'hello world', got %v", lines)
	}

	// Check log file was renamed.
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("expected notify.log to be renamed after migration")
	}
	if _, err := os.Stat(logPath + ".migrated"); os.IsNotExist(err) {
		t.Fatal("expected notify.log.migrated to exist")
	}
}

func TestSQLiteStoreMigrationSkipsWhenNoLog(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "notify.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	entries, _ := s.Entries(0)
	if entries != nil {
		t.Fatalf("expected nil entries with no log to migrate, got %v", entries)
	}
}

func TestSQLiteStoreReadContentExecution(t *testing.T) {
	s := tempSQLiteStore(t)

	vars := tmpl.Vars{Profile: "test"}
	steps := []config.Step{{Type: "sound", Sound: "blip"}, {Type: "say", Text: "hello"}}

	s.Log("ready", steps, true, vars, nil)

	content, err := s.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "profile=test") {
		t.Fatal("expected profile=test in content")
	}
	if !strings.Contains(content, "action=ready") {
		t.Fatal("expected action=ready in content")
	}
	if !strings.Contains(content, "afk=true") {
		t.Fatal("expected afk=true in content")
	}
	if !strings.Contains(content, "step[1] sound") {
		t.Fatal("expected step[1] sound in content")
	}
	if !strings.Contains(content, "step[2] say") {
		t.Fatal("expected step[2] say in content")
	}
}

func TestSQLiteStoreLogClaudeFields(t *testing.T) {
	s := tempSQLiteStore(t)

	vars := tmpl.Vars{Profile: "test", ClaudeHook: "post_tool_use", ClaudeMessage: "task done"}
	steps := []config.Step{{Type: "sound", Sound: "blip"}}

	s.Log("ready", steps, false, vars, nil)

	entries, _ := s.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ClaudeHook != "post_tool_use" {
		t.Fatalf("expected claude_hook 'post_tool_use', got %q", entries[0].ClaudeHook)
	}
	if entries[0].ClaudeMessage != "task done" {
		t.Fatalf("expected claude_message 'task done', got %q", entries[0].ClaudeMessage)
	}

	content, _ := s.ReadContent()
	if !strings.Contains(content, "claude_hook=post_tool_use") {
		t.Fatal("expected claude_hook in content")
	}
	if !strings.Contains(content, `claude_message="task done"`) {
		t.Fatal("expected claude_message in content")
	}
}
