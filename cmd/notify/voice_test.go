package main

import (
	"strings"
	"testing"

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
