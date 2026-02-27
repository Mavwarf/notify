package eventlog

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/paths"
	"github.com/Mavwarf/notify/internal/tmpl"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct {
	db   *sql.DB
	path string
}

// NewSQLiteStore opens (or creates) a SQLite database at path, creates
// tables and indexes, and performs one-time migration from notify.log
// if it exists in the same directory.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), paths.DirPerm); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Set PRAGMAs before any DDL.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite pragma: %w", err)
		}
	}

	ddl := `
CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       TEXT    NOT NULL,
    profile         TEXT    NOT NULL DEFAULT '',
    action          TEXT    NOT NULL DEFAULT '',
    kind            INTEGER NOT NULL,
    afk             INTEGER NOT NULL DEFAULT 0,
    desktop         INTEGER,
    claude_hook     TEXT    NOT NULL DEFAULT '',
    claude_message  TEXT    NOT NULL DEFAULT '',
    steps_csv       TEXT    NOT NULL DEFAULT '',
    extra           TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS step_details (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id   INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    step_num   INTEGER NOT NULL,
    step_type  TEXT    NOT NULL,
    detail     TEXT    NOT NULL,
    voice_text TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_profile   ON events(profile, action);
CREATE INDEX IF NOT EXISTS idx_step_voice       ON step_details(voice_text)
    WHERE voice_text IS NOT NULL;
`
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite schema: %w", err)
	}

	s := &SQLiteStore{db: db, path: path}

	// One-time migration from flat file.
	logPath := filepath.Join(filepath.Dir(path), paths.LogFileName)
	if _, err := os.Stat(logPath); err == nil {
		if err := s.migrateFromFile(logPath); err != nil {
			fmt.Fprintf(os.Stderr, "eventlog: migration: %v\n", err)
		}
	}

	return s, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// voiceTextTypes lists step types that carry TTS voice text.
var voiceTextTypes = map[string]bool{
	"say": true, "discord_voice": true, "telegram_audio": true, "telegram_voice": true,
}

func (s *SQLiteStore) Log(action string, steps []config.Step, afk bool, vars tmpl.Vars, desktop *int) error {
	ts := time.Now().Format(time.RFC3339)

	types := make([]string, len(steps))
	for i, st := range steps {
		types[i] = st.Type
	}

	afkInt := 0
	if afk {
		afkInt = 1
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var desktopVal any
	if desktop != nil {
		desktopVal = *desktop
	}

	res, err := tx.Exec(
		`INSERT INTO events (timestamp, profile, action, kind, afk, desktop, claude_hook, claude_message, steps_csv)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, vars.Profile, action, int(KindExecution), afkInt, desktopVal,
		vars.ClaudeHook, vars.ClaudeMessage, strings.Join(types, ","),
	)
	if err != nil {
		return err
	}

	eventID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for i, st := range steps {
		detail := StepSummary(st, &vars)
		var voiceText *string
		if voiceTextTypes[st.Type] {
			expanded := tmpl.Expand(st.Text, vars)
			voiceText = &expanded
		}
		if _, err := tx.Exec(
			`INSERT INTO step_details (event_id, step_num, step_type, detail, voice_text)
			 VALUES (?, ?, ?, ?, ?)`,
			eventID, i+1, st.Type, detail, voiceText,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) LogCooldown(profile, action string, seconds int) error {
	ts := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO events (timestamp, profile, action, kind, extra) VALUES (?, ?, ?, ?, ?)`,
		ts, profile, action, int(KindCooldown), fmt.Sprintf("skipped (%ds)", seconds),
	)
	return err
}

func (s *SQLiteStore) LogCooldownRecord(profile, action string, seconds int) error {
	ts := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO events (timestamp, profile, action, kind, extra) VALUES (?, ?, ?, ?, ?)`,
		ts, profile, action, int(KindOther), fmt.Sprintf("recorded (%ds)", seconds),
	)
	return err
}

func (s *SQLiteStore) LogSilent(profile, action string) error {
	ts := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO events (timestamp, profile, action, kind) VALUES (?, ?, ?, ?)`,
		ts, profile, action, int(KindSilent),
	)
	return err
}

func (s *SQLiteStore) LogSilentEnable(d time.Duration) error {
	ts := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO events (timestamp, kind, extra) VALUES (?, ?, ?)`,
		ts, int(KindOther), fmt.Sprintf("enabled (%s)", d),
	)
	return err
}

func (s *SQLiteStore) LogSilentDisable() error {
	ts := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO events (timestamp, kind, extra) VALUES (?, ?, ?)`,
		ts, int(KindOther), "disabled",
	)
	return err
}

func (s *SQLiteStore) Entries(days int) ([]Entry, error) {
	query := `SELECT timestamp, profile, action, kind, claude_hook, claude_message
		FROM events WHERE (profile != '' OR action != '')`
	var args []any
	if days > 0 {
		cutoff := DayCutoff(days).Format(time.RFC3339)
		query += ` AND timestamp >= ?`
		args = append(args, cutoff)
	}
	query += ` ORDER BY id`

	return s.queryEntries(query, args...)
}

func (s *SQLiteStore) EntriesSince(cutoff time.Time) ([]Entry, error) {
	query := `SELECT timestamp, profile, action, kind, claude_hook, claude_message
		FROM events WHERE timestamp >= ? AND (profile != '' OR action != '')
		ORDER BY id`
	return s.queryEntries(query, cutoff.Format(time.RFC3339))
}

func (s *SQLiteStore) queryEntries(query string, args ...any) ([]Entry, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var tsStr, profile, action, claudeHook, claudeMessage string
		var kind int
		if err := rows.Scan(&tsStr, &profile, &action, &kind, &claudeHook, &claudeMessage); err != nil {
			return nil, err
		}
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Time:          ts,
			Profile:       profile,
			Action:        action,
			Kind:          EntryKind(kind),
			ClaudeHook:    claudeHook,
			ClaudeMessage: claudeMessage,
		})
	}
	return entries, rows.Err()
}

func (s *SQLiteStore) VoiceLines(days int) ([]VoiceLine, error) {
	query := `SELECT sd.voice_text, COUNT(*) as cnt
		FROM step_details sd
		JOIN events e ON sd.event_id = e.id
		WHERE sd.voice_text IS NOT NULL`
	var args []any
	if days > 0 {
		cutoff := DayCutoff(days).Format(time.RFC3339)
		query += ` AND e.timestamp >= ?`
		args = append(args, cutoff)
	}
	query += ` GROUP BY sd.voice_text ORDER BY cnt DESC, sd.voice_text`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []VoiceLine
	for rows.Next() {
		var text string
		var count int
		if err := rows.Scan(&text, &count); err != nil {
			return nil, err
		}
		lines = append(lines, VoiceLine{Text: text, Count: count})
	}
	return lines, rows.Err()
}

func (s *SQLiteStore) ReadContent() (string, error) {
	rows, err := s.db.Query(
		`SELECT e.id, e.timestamp, e.profile, e.action, e.kind, e.afk,
		        e.desktop, e.claude_hook, e.claude_message, e.steps_csv, e.extra
		 FROM events e ORDER BY e.id`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type eventRow struct {
		id            int64
		ts            string
		profile       string
		action        string
		kind          int
		afk           int
		desktop       *int
		claudeHook    string
		claudeMessage string
		stepsCSV      string
		extra         string
	}

	var events []eventRow
	for rows.Next() {
		var ev eventRow
		if err := rows.Scan(&ev.id, &ev.ts, &ev.profile, &ev.action, &ev.kind, &ev.afk,
			&ev.desktop, &ev.claudeHook, &ev.claudeMessage, &ev.stepsCSV, &ev.extra); err != nil {
			return "", err
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(events) == 0 {
		return "", nil
	}

	// Fetch all step_details keyed by event_id.
	type stepRow struct {
		stepNum   int
		stepType  string
		detail    string
		voiceText *string
	}
	stepsByEvent := map[int64][]stepRow{}

	sdRows, err := s.db.Query(
		`SELECT event_id, step_num, step_type, detail, voice_text
		 FROM step_details ORDER BY event_id, step_num`)
	if err != nil {
		return "", err
	}
	defer sdRows.Close()

	for sdRows.Next() {
		var eventID int64
		var sr stepRow
		if err := sdRows.Scan(&eventID, &sr.stepNum, &sr.stepType, &sr.detail, &sr.voiceText); err != nil {
			return "", err
		}
		stepsByEvent[eventID] = append(stepsByEvent[eventID], sr)
	}
	if err := sdRows.Err(); err != nil {
		return "", err
	}

	// Reconstruct text.
	var b strings.Builder
	for _, ev := range events {
		switch EntryKind(ev.kind) {
		case KindExecution:
			summary := fmt.Sprintf("%s  profile=%s  action=%s  steps=%s  afk=%t",
				ev.ts, ev.profile, ev.action, ev.stepsCSV, ev.afk != 0)
			if ev.desktop != nil {
				summary += fmt.Sprintf("  desktop=%d", *ev.desktop)
			}
			if ev.claudeHook != "" {
				summary += fmt.Sprintf("  claude_hook=%s", ev.claudeHook)
			}
			if ev.claudeMessage != "" {
				summary += fmt.Sprintf("  claude_message=%q", ev.claudeMessage)
			}
			b.WriteString(summary)
			b.WriteByte('\n')
			for _, sr := range stepsByEvent[ev.id] {
				fmt.Fprintf(&b, "%s    step[%d] %s  %s\n", ev.ts, sr.stepNum, sr.stepType, sr.detail)
			}
			b.WriteByte('\n')

		case KindCooldown:
			fmt.Fprintf(&b, "%s  profile=%s  action=%s  cooldown=%s\n\n",
				ev.ts, ev.profile, ev.action, ev.extra)

		case KindSilent:
			fmt.Fprintf(&b, "%s  profile=%s  action=%s  silent=skipped\n\n",
				ev.ts, ev.profile, ev.action)

		case KindOther:
			if ev.profile != "" || ev.action != "" {
				// cooldown=recorded
				fmt.Fprintf(&b, "%s  profile=%s  action=%s  cooldown=%s\n",
					ev.ts, ev.profile, ev.action, ev.extra)
			} else if strings.HasPrefix(ev.extra, "enabled") {
				fmt.Fprintf(&b, "%s  silent=%s\n\n", ev.ts, ev.extra)
			} else if ev.extra == "disabled" {
				fmt.Fprintf(&b, "%s  silent=disabled\n\n", ev.ts)
			}
		}
	}

	return b.String(), nil
}

func (s *SQLiteStore) Clean(days int) (int, error) {
	cutoff := DayCutoff(days).Format(time.RFC3339)
	res, err := s.db.Exec(`DELETE FROM events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *SQLiteStore) RemoveProfile(name string) (int, error) {
	res, err := s.db.Exec(`DELETE FROM events WHERE profile = ?`, name)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *SQLiteStore) Clear() error {
	_, err := s.db.Exec(`DELETE FROM events`)
	return err
}

func (s *SQLiteStore) Path() string {
	return s.path
}

// migrateFromFile reads an existing notify.log and imports its entries into
// the SQLite database. On success, renames the log to notify.log.migrated.
func (s *SQLiteStore) migrateFromFile(logPath string) error {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		return os.Rename(logPath, logPath+".migrated")
	}

	blocks := SplitBlocks(content)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	migrated := 0
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}

		firstLine := lines[0]
		ts, ok := ExtractTimestamp(firstLine)
		if !ok {
			continue
		}
		tsStr := ts.Format(time.RFC3339)

		profile := extractField(firstLine, "profile")
		action := extractField(firstLine, "action")

		// Classify the block.
		kind := KindOther
		extra := ""
		stepsCSV := ""
		afk := false
		var desktop *int
		claudeHook := ""
		claudeMessage := ""

		if hasField(firstLine, "steps") {
			kind = KindExecution
			stepsCSV = extractField(firstLine, "steps")
			afk = extractField(firstLine, "afk") == "true"
			if d := extractField(firstLine, "desktop"); d != "" {
				var dv int
				if _, err := fmt.Sscanf(d, "%d", &dv); err == nil {
					desktop = &dv
				}
			}
			claudeHook = extractField(firstLine, "claude_hook")
			claudeMessage = extractQuotedField(firstLine, "claude_message")
		} else if cooldownVal := extractField(firstLine, "cooldown"); cooldownVal != "" {
			if strings.HasPrefix(cooldownVal, "skipped") {
				kind = KindCooldown
			}
			extra = cooldownVal
		} else if silentVal := extractField(firstLine, "silent"); silentVal != "" {
			if silentVal == "skipped" {
				kind = KindSilent
			} else {
				kind = KindOther
				extra = silentVal
			}
		}

		afkInt := 0
		if afk {
			afkInt = 1
		}
		var desktopVal any
		if desktop != nil {
			desktopVal = *desktop
		}

		res, err := tx.Exec(
			`INSERT INTO events (timestamp, profile, action, kind, afk, desktop, claude_hook, claude_message, steps_csv, extra)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tsStr, profile, action, int(kind), afkInt, desktopVal,
			claudeHook, claudeMessage, stepsCSV, extra,
		)
		if err != nil {
			return fmt.Errorf("migrate event: %w", err)
		}

		eventID, _ := res.LastInsertId()

		// Parse step detail lines for execution blocks.
		if kind == KindExecution {
			for _, line := range lines[1:] {
				stepNum, stepType, detail, voiceText := parseStepLine(line)
				if stepType == "" {
					continue
				}
				if _, err := tx.Exec(
					`INSERT INTO step_details (event_id, step_num, step_type, detail, voice_text)
					 VALUES (?, ?, ?, ?, ?)`,
					eventID, stepNum, stepType, detail, voiceText,
				); err != nil {
					return fmt.Errorf("migrate step: %w", err)
				}
			}
		}

		migrated++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "eventlog: migrated %d entries from notify.log\n", migrated)
	return os.Rename(logPath, logPath+".migrated")
}

// parseStepLine extracts step number, type, detail, and optional voice text
// from a log step detail line like:
//
//	2025-01-01T00:00:00Z    step[1] say  text="hello"
func parseStepLine(line string) (num int, stepType, detail string, voiceText *string) {
	idx := strings.Index(line, "step[")
	if idx < 0 {
		return 0, "", "", nil
	}

	after := line[idx+5:] // after "step["
	bracket := strings.Index(after, "]")
	if bracket < 0 {
		return 0, "", "", nil
	}

	fmt.Sscanf(after[:bracket], "%d", &num)

	// After "] " is "type  detail"
	rest := after[bracket+1:]
	rest = strings.TrimLeft(rest, " ")

	// Step type is the first word.
	spaceIdx := strings.Index(rest, " ")
	if spaceIdx < 0 {
		stepType = rest
		return num, stepType, "", nil
	}
	stepType = rest[:spaceIdx]
	detail = strings.TrimLeft(rest[spaceIdx:], " ")

	// Extract voice text for TTS step types.
	if voiceTextTypes[stepType] {
		for _, suffix := range voiceStepSuffixes {
			sIdx := strings.Index(line, suffix)
			if sIdx >= 0 {
				raw := line[sIdx+len(suffix):]
				text := extractQuoted(raw)
				if text != "" {
					voiceText = &text
				}
				break
			}
		}
	}

	return num, stepType, detail, voiceText
}
