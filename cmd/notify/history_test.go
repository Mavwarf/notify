package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/eventlog"
)

func init() {
	// Disable ANSI colors so test output is deterministic.
	noColor = true
}

// --- fmtNum ---

func TestFmtNum(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{5, "5"},
		{999, "999"},
		{1000, "1.000"},
		{12345, "12.345"},
		{1234567, "1.234.567"},
		{-42, "-42"},
		{-1500, "-1.500"},
	}
	for _, tt := range tests {
		if got := fmtNum(tt.n); got != tt.want {
			t.Errorf("fmtNum(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// --- fmtPct ---

func TestFmtPct(t *testing.T) {
	tests := []struct {
		n, total int
		want     string
	}{
		{50, 100, "50%"},
		{1, 3, "33%"},
		{2, 3, "66%"},
		{100, 100, "100%"},
		{0, 100, "0%"},
		{0, 0, ""},
		{5, 0, ""},
	}
	for _, tt := range tests {
		if got := fmtPct(tt.n, tt.total); got != tt.want {
			t.Errorf("fmtPct(%d, %d) = %q, want %q", tt.n, tt.total, got, tt.want)
		}
	}
}

// --- aggregateGroups ---

func TestAggregateGroupsSingle(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "boss", Action: "done", Executions: 10, Skipped: 2},
			{Profile: "boss", Action: "alert", Executions: 3, Skipped: 0},
			{Profile: "dev", Action: "done", Executions: 5, Skipped: 0},
		},
	}}

	td := aggregateGroups(groups)

	// Profile order is sorted alphabetically.
	if len(td.profileOrder) != 2 || td.profileOrder[0] != "boss" || td.profileOrder[1] != "dev" {
		t.Errorf("profileOrder = %v, want [boss dev]", td.profileOrder)
	}

	// Per-profile totals.
	boss := td.perProfile["boss"]
	if boss.exec != 13 || boss.skip != 2 {
		t.Errorf("boss = exec:%d skip:%d, want exec:13 skip:2", boss.exec, boss.skip)
	}
	dev := td.perProfile["dev"]
	if dev.exec != 5 || dev.skip != 0 {
		t.Errorf("dev = exec:%d skip:%d, want exec:5 skip:0", dev.exec, dev.skip)
	}

	// hasSkipped should be true (boss/done has skips).
	if !td.hasSkipped {
		t.Error("hasSkipped = false, want true")
	}
}

func TestAggregateGroupsMultipleDays(t *testing.T) {
	groups := []eventlog.DayGroup{
		{
			Date: time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC),
			Summaries: []eventlog.DaySummary{
				{Profile: "app", Action: "ready", Executions: 3, Skipped: 0},
			},
		},
		{
			Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
			Summaries: []eventlog.DaySummary{
				{Profile: "app", Action: "ready", Executions: 7, Skipped: 1},
			},
		},
	}

	td := aggregateGroups(groups)

	app := td.perProfile["app"]
	if app.exec != 10 || app.skip != 1 {
		t.Errorf("app = exec:%d skip:%d, want exec:10 skip:1", app.exec, app.skip)
	}

	ak := actionKey{"app", "ready"}
	ac := td.perAction[ak]
	if ac.exec != 10 || ac.skip != 1 {
		t.Errorf("app/ready = exec:%d skip:%d, want exec:10 skip:1", ac.exec, ac.skip)
	}
}

func TestAggregateGroupsNoSkips(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "x", Action: "a", Executions: 1, Skipped: 0},
		},
	}}
	td := aggregateGroups(groups)
	if td.hasSkipped {
		t.Error("hasSkipped = true, want false")
	}
}

// --- buildBaseline ---

func TestBuildBaseline(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "boss", Action: "done", Executions: 10, Skipped: 2},
			{Profile: "dev", Action: "ready", Executions: 5, Skipped: 0},
		},
	}}

	b := buildBaseline(groups)

	if got := b["boss/done"]; got != 12 {
		t.Errorf("boss/done = %d, want 12", got)
	}
	if got := b["dev/ready"]; got != 5 {
		t.Errorf("dev/ready = %d, want 5", got)
	}
	if got := b["missing/key"]; got != 0 {
		t.Errorf("missing/key = %d, want 0", got)
	}
}

func TestBuildBaselineEmpty(t *testing.T) {
	b := buildBaseline(nil)
	if len(b) != 0 {
		t.Errorf("len = %d, want 0", len(b))
	}
}

// --- renderSummaryTable ---

func TestRenderSummaryTableBasic(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "boss", Action: "done", Executions: 8, Skipped: 0},
			{Profile: "boss", Action: "alert", Executions: 2, Skipped: 0},
			{Profile: "dev", Action: "done", Executions: 10, Skipped: 0},
		},
	}}

	var out strings.Builder
	renderSummaryTable(&out, groups, nil)
	s := out.String()

	// Date header.
	if !strings.Contains(s, "2026-02-24") {
		t.Error("missing date header")
	}
	// Column headers.
	if !strings.Contains(s, "Total") {
		t.Error("missing Total header")
	}
	if !strings.Contains(s, "%") {
		t.Error("missing % header")
	}
	// No Skipped column when there are no skips.
	if strings.Contains(s, "Skipped") {
		t.Error("unexpected Skipped column with no skips")
	}
	// Profile names.
	if !strings.Contains(s, "boss") {
		t.Error("missing boss profile")
	}
	if !strings.Contains(s, "dev") {
		t.Error("missing dev profile")
	}
	// Percentage values: boss=10/20=50%, dev=10/20=50%.
	if !strings.Contains(s, "50%") {
		t.Errorf("missing expected 50%% in output:\n%s", s)
	}
	// Grand total.
	if !strings.Contains(s, "20") {
		t.Error("missing grand total 20")
	}
}

func TestRenderSummaryTableWithSkips(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "boss", Action: "done", Executions: 7, Skipped: 3},
		},
	}}

	var out strings.Builder
	renderSummaryTable(&out, groups, nil)
	s := out.String()

	if !strings.Contains(s, "Skipped") {
		t.Error("missing Skipped column header")
	}
	if !strings.Contains(s, "3") {
		t.Error("missing skipped count 3")
	}
}

func TestRenderSummaryTableWithBaseline(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "app", Action: "ready", Executions: 15, Skipped: 0},
		},
	}}
	baseline := map[string]int{"app/ready": 10}

	var out strings.Builder
	renderSummaryTable(&out, groups, baseline)
	s := out.String()

	if !strings.Contains(s, "New") {
		t.Error("missing New column header")
	}
	// New delta: 15 - 10 = +5.
	if !strings.Contains(s, "+5") {
		t.Errorf("missing +5 delta in output:\n%s", s)
	}
}

func TestRenderSummaryTableMultiDay(t *testing.T) {
	groups := []eventlog.DayGroup{
		{
			Date: time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC),
			Summaries: []eventlog.DaySummary{
				{Profile: "a", Action: "x", Executions: 1, Skipped: 0},
			},
		},
		{
			Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
			Summaries: []eventlog.DaySummary{
				{Profile: "a", Action: "x", Executions: 2, Skipped: 0},
			},
		},
	}

	var out strings.Builder
	renderSummaryTable(&out, groups, nil)
	s := out.String()

	// Multi-day header shows date range.
	if !strings.Contains(s, "2026-02-23") || !strings.Contains(s, "2026-02-24") {
		t.Errorf("missing date range in header:\n%s", s)
	}
	// Grand total should be 3.
	if !strings.Contains(s, "3") {
		t.Error("missing grand total 3")
	}
}

func TestRenderSummaryTablePercentageSingleProfile(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "only", Action: "act", Executions: 42, Skipped: 0},
		},
	}}

	var out strings.Builder
	renderSummaryTable(&out, groups, nil)
	s := out.String()

	// Single profile should show 100%.
	if !strings.Contains(s, "100%") {
		t.Errorf("missing 100%% for single profile:\n%s", s)
	}
}
