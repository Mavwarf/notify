package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/eventlog"
)

func TestVoiceStatsRendering(t *testing.T) {
	// Disable colors for predictable output.
	origNoColor := noColor
	noColor = true
	defer func() { noColor = origNoColor }()

	lines := []eventlog.VoiceLine{
		{Text: "Boss done", Count: 312},
		{Text: "Romans done", Count: 245},
		{Text: "Build complete", Count: 120},
	}

	var out strings.Builder
	renderVoiceTable(&out, lines, 0)
	result := out.String()

	// Verify header.
	if !strings.Contains(result, "all time") {
		t.Errorf("expected header to contain 'all time', got:\n%s", result)
	}
	if !strings.Contains(result, "677 total") {
		t.Errorf("expected header to contain '677 total', got:\n%s", result)
	}

	// Verify column headers.
	if !strings.Contains(result, "#") || !strings.Contains(result, "Count") || !strings.Contains(result, "Text") {
		t.Errorf("expected column headers, got:\n%s", result)
	}

	// Verify data rows.
	if !strings.Contains(result, "Boss done") {
		t.Errorf("expected 'Boss done' in output, got:\n%s", result)
	}
	if !strings.Contains(result, "312") {
		t.Errorf("expected '312' in output, got:\n%s", result)
	}
	if !strings.Contains(result, "46%") {
		t.Errorf("expected '46%%' in output, got:\n%s", result)
	}

	// Verify rank numbering.
	resultLines := strings.Split(result, "\n")
	foundRank1, foundRank3 := false, false
	for _, line := range resultLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "1") && strings.Contains(line, "Boss done") {
			foundRank1 = true
		}
		if strings.HasPrefix(trimmed, "3") && strings.Contains(line, "Build complete") {
			foundRank3 = true
		}
	}
	if !foundRank1 {
		t.Error("expected rank 1 for 'Boss done'")
	}
	if !foundRank3 {
		t.Error("expected rank 3 for 'Build complete'")
	}
}

func TestVoiceStatsRenderingWithDays(t *testing.T) {
	origNoColor := noColor
	noColor = true
	defer func() { noColor = origNoColor }()

	lines := []eventlog.VoiceLine{
		{Text: "Ready", Count: 10},
	}

	var out strings.Builder
	renderVoiceTable(&out, lines, 7)
	result := out.String()

	if !strings.Contains(result, "last 7 days") {
		t.Errorf("expected header to contain 'last 7 days', got:\n%s", result)
	}
}

// --- filterContentByDays ---

// makeLogBlock creates a log block with a given timestamp for testing.
func makeLogBlock(t time.Time, profile, action string) string {
	return fmt.Sprintf("%s  profile=%s action=%s\nsteps: [sound]", t.Format(time.RFC3339), profile, action)
}

func TestFilterContentByDaysKeepsRecent(t *testing.T) {
	now := time.Now()
	today := makeLogBlock(now, "app", "ready")
	yesterday := makeLogBlock(now.AddDate(0, 0, -1), "app", "done")

	content := today + "\n\n" + yesterday
	result := filterContentByDays(content, 7)

	if !strings.Contains(result, "ready") {
		t.Error("should keep today's entry")
	}
	if !strings.Contains(result, "done") {
		t.Error("should keep yesterday's entry")
	}
}

func TestFilterContentByDaysRemovesOld(t *testing.T) {
	now := time.Now()
	today := makeLogBlock(now, "app", "ready")
	old := makeLogBlock(now.AddDate(0, 0, -30), "app", "ancient")

	content := today + "\n\n" + old
	result := filterContentByDays(content, 7)

	if !strings.Contains(result, "ready") {
		t.Error("should keep today's entry")
	}
	if strings.Contains(result, "ancient") {
		t.Error("should remove 30-day-old entry when filtering to 7 days")
	}
}

func TestFilterContentByDaysAllOld(t *testing.T) {
	now := time.Now()
	old1 := makeLogBlock(now.AddDate(0, 0, -10), "app", "old1")
	old2 := makeLogBlock(now.AddDate(0, 0, -20), "app", "old2")

	content := old1 + "\n\n" + old2
	result := filterContentByDays(content, 3)

	if strings.TrimSpace(result) != "" {
		t.Errorf("should return empty for all old entries, got: %q", result)
	}
}

func TestFilterContentByDaysEdgeCutoff(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Entry from exactly N-1 days ago (should be kept with days=N).
	boundary := makeLogBlock(today.AddDate(0, 0, -6), "app", "boundary")
	// Entry from exactly N days ago (should be removed with days=N).
	outside := makeLogBlock(today.AddDate(0, 0, -7), "app", "outside")

	content := boundary + "\n\n" + outside
	result := filterContentByDays(content, 7)

	if !strings.Contains(result, "boundary") {
		t.Error("entry at cutoff boundary should be kept")
	}
	if strings.Contains(result, "outside") {
		t.Error("entry beyond cutoff should be removed")
	}
}

func TestFilterContentByDaysOneDayKeepsToday(t *testing.T) {
	now := time.Now()
	today := makeLogBlock(now, "app", "today")
	yesterday := makeLogBlock(now.AddDate(0, 0, -1), "app", "yesterday")

	content := today + "\n\n" + yesterday
	result := filterContentByDays(content, 1)

	if !strings.Contains(result, "today") {
		t.Error("days=1 should keep today's entry")
	}
	if strings.Contains(result, "yesterday") {
		t.Error("days=1 should remove yesterday's entry")
	}
}

func TestFilterContentByDaysEmptyContent(t *testing.T) {
	result := filterContentByDays("", 7)
	if strings.TrimSpace(result) != "" {
		t.Errorf("empty content should return empty, got: %q", result)
	}
}

func TestFilterContentByDaysMalformedBlock(t *testing.T) {
	now := time.Now()
	good := makeLogBlock(now, "app", "ready")
	bad := "not-a-timestamp  some data"

	content := good + "\n\n" + bad
	result := filterContentByDays(content, 7)

	if !strings.Contains(result, "ready") {
		t.Error("valid block should be kept")
	}
	if strings.Contains(result, "not-a-timestamp") {
		t.Error("malformed block should be dropped")
	}
}
