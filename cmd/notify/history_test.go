package main

import (
	"fmt"
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

// --- AggregateGroups (now in eventlog package, tested via exported API) ---

func TestAggregateGroupsSingle(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "boss", Action: "done", Executions: 10, Skipped: 2},
			{Profile: "boss", Action: "alert", Executions: 3, Skipped: 0},
			{Profile: "dev", Action: "done", Executions: 5, Skipped: 0},
		},
	}}

	ad := eventlog.AggregateGroups(groups)

	// Profile order is sorted alphabetically.
	if len(ad.ProfileOrder) != 2 || ad.ProfileOrder[0] != "boss" || ad.ProfileOrder[1] != "dev" {
		t.Errorf("ProfileOrder = %v, want [boss dev]", ad.ProfileOrder)
	}

	// Per-profile totals.
	boss := ad.PerProfile["boss"]
	if boss.Exec != 13 || boss.Skip != 2 {
		t.Errorf("boss = exec:%d skip:%d, want exec:13 skip:2", boss.Exec, boss.Skip)
	}
	dev := ad.PerProfile["dev"]
	if dev.Exec != 5 || dev.Skip != 0 {
		t.Errorf("dev = exec:%d skip:%d, want exec:5 skip:0", dev.Exec, dev.Skip)
	}

	// HasSkipped should be true (boss/done has skips).
	if !ad.HasSkipped {
		t.Error("HasSkipped = false, want true")
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

	ad := eventlog.AggregateGroups(groups)

	app := ad.PerProfile["app"]
	if app.Exec != 10 || app.Skip != 1 {
		t.Errorf("app = exec:%d skip:%d, want exec:10 skip:1", app.Exec, app.Skip)
	}

	ak := eventlog.ActionKey{Profile: "app", Action: "ready"}
	ac := ad.PerAction[ak]
	if ac.Exec != 10 || ac.Skip != 1 {
		t.Errorf("app/ready = exec:%d skip:%d, want exec:10 skip:1", ac.Exec, ac.Skip)
	}
}

func TestAggregateGroupsNoSkips(t *testing.T) {
	groups := []eventlog.DayGroup{{
		Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
		Summaries: []eventlog.DaySummary{
			{Profile: "x", Action: "a", Executions: 1, Skipped: 0},
		},
	}}
	ad := eventlog.AggregateGroups(groups)
	if ad.HasSkipped {
		t.Error("HasSkipped = true, want false")
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

// --- renderHourlyTable ---

// mkEntry builds an eventlog.Entry dated today at the given hour.
func mkEntry(profile, action string, hour int) eventlog.Entry {
	now := time.Now()
	return eventlog.Entry{
		Time:    time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location()),
		Profile: profile,
		Action:  action,
		Kind:    eventlog.KindExecution,
	}
}

func TestRenderHourlyTableBasic(t *testing.T) {
	entries := []eventlog.Entry{
		mkEntry("boss", "done", 9),
		mkEntry("boss", "done", 9),
		mkEntry("dev", "ready", 11),
		mkEntry("dev", "ready", 11),
		mkEntry("dev", "ready", 11),
	}

	var out strings.Builder
	renderHourlyTable(&out, entries)
	s := out.String()

	// Column headers.
	for _, want := range []string{"Hour", "Total", "%"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q header in output:\n%s", want, s)
		}
	}
	// Profile names.
	for _, want := range []string{"boss", "dev"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing profile %q in output:\n%s", want, s)
		}
	}
	// Hour rows.
	if !strings.Contains(s, "09:00") {
		t.Errorf("missing 09:00 row:\n%s", s)
	}
	if !strings.Contains(s, "11:00") {
		t.Errorf("missing 11:00 row:\n%s", s)
	}
	// Gap hour 10:00 should appear with dashes.
	if !strings.Contains(s, "10:00") {
		t.Errorf("missing gap hour 10:00:\n%s", s)
	}
	// Total row.
	if !strings.Contains(s, "Total") {
		t.Errorf("missing Total row:\n%s", s)
	}
	// Grand total is 5.
	if !strings.Contains(s, "5") {
		t.Errorf("missing grand total 5:\n%s", s)
	}
}

func TestRenderHourlyTableSingleProfile(t *testing.T) {
	entries := []eventlog.Entry{
		mkEntry("app", "deploy", 14),
		mkEntry("app", "deploy", 14),
		mkEntry("app", "deploy", 16),
	}

	var out strings.Builder
	renderHourlyTable(&out, entries)
	s := out.String()

	if !strings.Contains(s, "app") {
		t.Errorf("missing profile app:\n%s", s)
	}
	// Grand total is 3.
	if !strings.Contains(s, "3") {
		t.Errorf("missing grand total 3:\n%s", s)
	}
	// 14:00 has 2/3 = 66%.
	if !strings.Contains(s, "66%") {
		t.Errorf("missing 66%% for hour 14:\n%s", s)
	}
	// 16:00 has 1/3 = 33%.
	if !strings.Contains(s, "33%") {
		t.Errorf("missing 33%% for hour 16:\n%s", s)
	}
}

func TestRenderHourlyTableEmpty(t *testing.T) {
	// Entries from yesterday should not match today's date filter.
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	entries := []eventlog.Entry{{
		Time:    time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 10, 0, 0, 0, now.Location()),
		Profile: "old",
		Action:  "stale",
		Kind:    eventlog.KindExecution,
	}}

	var out strings.Builder
	renderHourlyTable(&out, entries)

	if out.Len() != 0 {
		t.Errorf("expected empty output for yesterday's entries, got:\n%s", out.String())
	}
}

func TestRenderHourlyTableSingleHour(t *testing.T) {
	entries := []eventlog.Entry{
		mkEntry("ci", "build", 8),
		mkEntry("ci", "build", 8),
		mkEntry("ci", "build", 8),
	}

	var out strings.Builder
	renderHourlyTable(&out, entries)
	s := out.String()

	// Only one hour row.
	if !strings.Contains(s, "08:00") {
		t.Errorf("missing 08:00 row:\n%s", s)
	}
	// That single hour should be 100%.
	if !strings.Contains(s, "100%") {
		t.Errorf("missing 100%% for single hour:\n%s", s)
	}
	// No other hour rows should exist (spot-check an adjacent hour).
	if strings.Contains(s, "07:00") || strings.Contains(s, "09:00") {
		t.Errorf("unexpected extra hour rows:\n%s", s)
	}
}

func TestRenderHourlyTableGapHours(t *testing.T) {
	entries := []eventlog.Entry{
		mkEntry("web", "ping", 6),
		mkEntry("web", "ping", 6),
		mkEntry("web", "ping", 10),
	}

	var out strings.Builder
	renderHourlyTable(&out, entries)
	s := out.String()

	// Rows for hours 6 through 10.
	for h := 6; h <= 10; h++ {
		label := fmt.Sprintf("%02d:00", h)
		if !strings.Contains(s, label) {
			t.Errorf("missing hour row %s:\n%s", label, s)
		}
	}
	// Hours outside the range should not appear.
	if strings.Contains(s, "05:00") || strings.Contains(s, "11:00") {
		t.Errorf("unexpected hour rows outside 6-10 range:\n%s", s)
	}
	// Grand total is 3.
	lines := strings.Split(s, "\n")
	// Find the total row (last non-empty data line after separator).
	var totalLine string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Total") {
			totalLine = l
		}
	}
	if !strings.Contains(totalLine, "3") {
		t.Errorf("total row missing grand total 3:\n%s", totalLine)
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

// --- btoi ---

func TestBtoi(t *testing.T) {
	if got := btoi(true); got != 1 {
		t.Errorf("btoi(true) = %d, want 1", got)
	}
	if got := btoi(false); got != 0 {
		t.Errorf("btoi(false) = %d, want 0", got)
	}
}

// --- padL / padR ---

func TestPadL(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  string
	}{
		{"hi", 5, "   hi"},
		{"hello", 5, "hello"},
		{"toolong", 3, "toolong"},
		{"", 3, "   "},
	}
	for _, tt := range tests {
		if got := padL(tt.s, tt.width); got != tt.want {
			t.Errorf("padL(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}

func TestPadR(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"toolong", 3, "toolong"},
		{"", 3, "   "},
	}
	for _, tt := range tests {
		if got := padR(tt.s, tt.width); got != tt.want {
			t.Errorf("padR(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}

// --- ansi / color helpers ---

func TestAnsiWithColor(t *testing.T) {
	// Temporarily enable color for these tests.
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	got := ansi("\033[1m", "test")
	if got != "\033[1mtest\033[0m" {
		t.Errorf("ansi with color = %q, want ANSI-wrapped", got)
	}
}

func TestAnsiNoColor(t *testing.T) {
	orig := noColor
	noColor = true
	defer func() { noColor = orig }()

	if got := ansi("\033[1m", "test"); got != "test" {
		t.Errorf("ansi with noColor = %q, want \"test\"", got)
	}
}

func TestColorFunctions(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	tests := []struct {
		name string
		fn   func(string) string
		code string
	}{
		{"bold", bold, "\033[1m"},
		{"dim", dim, "\033[2m"},
		{"cyan", cyan, "\033[36m"},
		{"green", green, "\033[32m"},
		{"yellow", yellow, "\033[33m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("x")
			want := tt.code + "x" + "\033[0m"
			if got != want {
				t.Errorf("%s(\"x\") = %q, want %q", tt.name, got, want)
			}
		})
	}
}

// --- colorPadL ---

func TestColorPadL(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	// "hi" is 2 visible chars; with width=6 we want 4 spaces of padding.
	// The ANSI codes add invisible bytes, so total string is longer than 6.
	got := colorPadL(cyan, "hi", 6)
	if !strings.HasSuffix(got, "hi\033[0m") {
		t.Errorf("colorPadL should end with colored text, got %q", got)
	}
	// Visible content should be 4 spaces + "hi" = 6 chars.
	plain := strings.ReplaceAll(strings.ReplaceAll(got, "\033[36m", ""), "\033[0m", "")
	if plain != "    hi" {
		t.Errorf("colorPadL visible content = %q, want \"    hi\"", plain)
	}
}
