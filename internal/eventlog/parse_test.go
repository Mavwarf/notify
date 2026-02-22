package eventlog

import (
	"testing"
	"time"
)

func TestParseEntries_Execution(t *testing.T) {
	content := "2026-02-22T10:00:00+01:00  profile=default  action=ready  steps=sound,say  afk=false\n" +
		"2026-02-22T10:00:00+01:00    step[1] sound  sound=success\n" +
		"2026-02-22T10:00:00+01:00    step[2] say  text=\"Ready!\"\n"

	entries := ParseEntries(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Profile != "default" {
		t.Errorf("profile = %q, want %q", entries[0].Profile, "default")
	}
	if entries[0].Action != "ready" {
		t.Errorf("action = %q, want %q", entries[0].Action, "ready")
	}
	if entries[0].Kind != KindExecution {
		t.Errorf("kind = %d, want KindExecution (%d)", entries[0].Kind, KindExecution)
	}
}

func TestParseEntries_CooldownSkip(t *testing.T) {
	content := "2026-02-22T10:05:00+01:00  profile=default  action=ready  cooldown=skipped (30s)\n"

	entries := ParseEntries(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != KindCooldown {
		t.Errorf("kind = %d, want KindCooldown (%d)", entries[0].Kind, KindCooldown)
	}
}

func TestParseEntries_SilentSkip(t *testing.T) {
	content := "2026-02-22T10:10:00+01:00  profile=default  action=ready  silent=skipped\n"

	entries := ParseEntries(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != KindSilent {
		t.Errorf("kind = %d, want KindSilent (%d)", entries[0].Kind, KindSilent)
	}
}

func TestParseEntries_SilentEnableIgnored(t *testing.T) {
	content := "2026-02-22T10:00:00+01:00  silent=enabled (1h0m0s)\n"

	entries := ParseEntries(content)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (silent=enabled has no profile), got %d", len(entries))
	}
}

func TestParseEntries_SilentDisableIgnored(t *testing.T) {
	content := "2026-02-22T10:00:00+01:00  silent=disabled\n"

	entries := ParseEntries(content)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (silent=disabled has no profile), got %d", len(entries))
	}
}

func TestParseEntries_CooldownRecordedIgnored(t *testing.T) {
	// cooldown=recorded entries are KindOther (no steps=, cooldown!=skipped).
	// SummarizeByDay ignores KindOther, so they don't affect counts.
	content := "2026-02-22T10:00:00+01:00  profile=default  action=ready  cooldown=recorded (30s)\n"

	entries := ParseEntries(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != KindOther {
		t.Errorf("kind = %d, want KindOther (%d)", entries[0].Kind, KindOther)
	}
}

func TestParseEntries_CooldownRecordedWithExecution(t *testing.T) {
	// Real-world pattern: cooldown=recorded uses \n (no blank line), so it
	// shares a block with the execution entry. Both lines should be parsed.
	content := "2026-02-22T10:00:00+01:00  profile=default  action=ready  cooldown=recorded (10s)\n" +
		"2026-02-22T10:00:00+01:00  profile=default  action=ready  steps=sound,say  afk=false\n" +
		"2026-02-22T10:00:00+01:00    step[1] sound  sound=blip\n" +
		"2026-02-22T10:00:00+01:00    step[2] say  text=\"Ready!\"\n"

	entries := ParseEntries(content)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (recorded + execution), got %d", len(entries))
	}
	if entries[0].Kind != KindOther {
		t.Errorf("entry[0] kind = %d, want KindOther (%d)", entries[0].Kind, KindOther)
	}
	if entries[1].Kind != KindExecution {
		t.Errorf("entry[1] kind = %d, want KindExecution (%d)", entries[1].Kind, KindExecution)
	}
}

func TestParseEntries_MalformedTimestamp(t *testing.T) {
	content := "not-a-timestamp  profile=default  action=ready  steps=sound  afk=false\n"

	entries := ParseEntries(content)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for malformed timestamp, got %d", len(entries))
	}
}

func TestParseEntries_MissingDoubleSpace(t *testing.T) {
	content := "2026-02-22T10:00:00+01:00 profile=default action=ready steps=sound afk=false\n"

	entries := ParseEntries(content)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for missing double-space separator, got %d", len(entries))
	}
}

func TestParseEntries_Empty(t *testing.T) {
	entries := ParseEntries("")
	if entries != nil {
		t.Fatalf("expected nil for empty content, got %v", entries)
	}

	entries = ParseEntries("   \n\n  ")
	if entries != nil {
		t.Fatalf("expected nil for whitespace-only content, got %v", entries)
	}
}

func TestParseEntries_MultipleEntries(t *testing.T) {
	content := "2026-02-22T10:00:00+01:00  profile=default  action=ready  steps=sound  afk=false\n" +
		"2026-02-22T10:00:00+01:00    step[1] sound  sound=success\n" +
		"\n" +
		"2026-02-22T10:05:00+01:00  profile=boss  action=ready  cooldown=skipped (30s)\n" +
		"\n" +
		"2026-02-22T11:00:00+01:00  profile=default  action=error  steps=sound  afk=true\n" +
		"2026-02-22T11:00:00+01:00    step[1] sound  sound=error\n"

	entries := ParseEntries(content)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Kind != KindExecution {
		t.Errorf("entry[0] kind = %d, want KindExecution", entries[0].Kind)
	}
	if entries[1].Kind != KindCooldown {
		t.Errorf("entry[1] kind = %d, want KindCooldown", entries[1].Kind)
	}
	if entries[2].Kind != KindExecution {
		t.Errorf("entry[2] kind = %d, want KindExecution", entries[2].Kind)
	}
}

func TestSummarizeByDay_Grouping(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	entries := []Entry{
		{Time: today, Profile: "default", Action: "ready", Kind: KindExecution},
		{Time: today, Profile: "default", Action: "ready", Kind: KindExecution},
		{Time: today, Profile: "default", Action: "ready", Kind: KindCooldown},
		{Time: today, Profile: "boss", Action: "ready", Kind: KindExecution},
		{Time: yesterday, Profile: "default", Action: "ready", Kind: KindExecution},
	}

	groups := SummarizeByDay(entries, 7)
	if len(groups) != 2 {
		t.Fatalf("expected 2 day groups, got %d", len(groups))
	}

	// First group is today (descending order).
	todayGroup := groups[0]
	if len(todayGroup.Summaries) != 2 {
		t.Fatalf("today: expected 2 summaries, got %d", len(todayGroup.Summaries))
	}

	// Summaries are sorted alphabetically: boss/ready before default/ready.
	if todayGroup.Summaries[0].Profile != "boss" {
		t.Errorf("today[0] profile = %q, want %q", todayGroup.Summaries[0].Profile, "boss")
	}
	if todayGroup.Summaries[0].Executions != 1 {
		t.Errorf("today[0] executions = %d, want 1", todayGroup.Summaries[0].Executions)
	}

	if todayGroup.Summaries[1].Profile != "default" {
		t.Errorf("today[1] profile = %q, want %q", todayGroup.Summaries[1].Profile, "default")
	}
	if todayGroup.Summaries[1].Executions != 2 {
		t.Errorf("today[1] executions = %d, want 2", todayGroup.Summaries[1].Executions)
	}
	if todayGroup.Summaries[1].Skipped != 1 {
		t.Errorf("today[1] skipped = %d, want 1", todayGroup.Summaries[1].Skipped)
	}

	// Second group is yesterday.
	yesterdayGroup := groups[1]
	if len(yesterdayGroup.Summaries) != 1 {
		t.Fatalf("yesterday: expected 1 summary, got %d", len(yesterdayGroup.Summaries))
	}
	if yesterdayGroup.Summaries[0].Executions != 1 {
		t.Errorf("yesterday[0] executions = %d, want 1", yesterdayGroup.Summaries[0].Executions)
	}
}

func TestSummarizeByDay_DayFiltering(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	old := today.AddDate(0, 0, -10)

	entries := []Entry{
		{Time: today, Profile: "default", Action: "ready", Kind: KindExecution},
		{Time: old, Profile: "default", Action: "ready", Kind: KindExecution},
	}

	groups := SummarizeByDay(entries, 7)
	if len(groups) != 1 {
		t.Fatalf("expected 1 day group (old entry filtered), got %d", len(groups))
	}
}

func TestSummarizeByDay_KindOtherIgnored(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())

	entries := []Entry{
		{Time: today, Profile: "default", Action: "ready", Kind: KindOther},
	}

	groups := SummarizeByDay(entries, 7)
	if len(groups) != 0 {
		t.Fatalf("expected 0 day groups (KindOther ignored), got %d", len(groups))
	}
}

func TestSummarizeByDay_Empty(t *testing.T) {
	groups := SummarizeByDay(nil, 7)
	if len(groups) != 0 {
		t.Fatalf("expected 0 day groups for nil entries, got %d", len(groups))
	}
}

func TestSummarizeByDay_SilentCountsAsSkipped(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())

	entries := []Entry{
		{Time: today, Profile: "default", Action: "ready", Kind: KindExecution},
		{Time: today, Profile: "default", Action: "ready", Kind: KindSilent},
	}

	groups := SummarizeByDay(entries, 7)
	if len(groups) != 1 {
		t.Fatalf("expected 1 day group, got %d", len(groups))
	}
	s := groups[0].Summaries[0]
	if s.Executions != 1 {
		t.Errorf("executions = %d, want 1", s.Executions)
	}
	if s.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", s.Skipped)
	}
}

func TestExtractField(t *testing.T) {
	line := "2026-02-22T10:00:00+01:00  profile=default  action=ready  steps=sound,say  afk=false"

	tests := []struct {
		key, want string
	}{
		{"profile", "default"},
		{"action", "ready"},
		{"steps", "sound,say"},
		{"afk", "false"},
		{"missing", ""},
	}

	for _, tt := range tests {
		got := extractField(line, tt.key)
		if got != tt.want {
			t.Errorf("extractField(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
