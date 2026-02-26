package eventlog

import (
	"testing"
	"time"
)

// --- AggregateGroups ---

func TestAggregateGroupsEmpty(t *testing.T) {
	ad := AggregateGroups(nil)
	if len(ad.PerAction) != 0 {
		t.Errorf("PerAction len = %d, want 0", len(ad.PerAction))
	}
	if len(ad.ProfileOrder) != 0 {
		t.Errorf("ProfileOrder len = %d, want 0", len(ad.ProfileOrder))
	}
	if ad.HasSkipped {
		t.Error("HasSkipped = true, want false")
	}
}

func TestAggregateGroupsSingleProfile(t *testing.T) {
	groups := []DayGroup{{
		Date: time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC),
		Summaries: []DaySummary{
			{Profile: "app", Action: "done", Executions: 5, Skipped: 0},
			{Profile: "app", Action: "alert", Executions: 3, Skipped: 0},
		},
	}}

	ad := AggregateGroups(groups)

	if len(ad.ProfileOrder) != 1 || ad.ProfileOrder[0] != "app" {
		t.Errorf("ProfileOrder = %v, want [app]", ad.ProfileOrder)
	}

	pc := ad.PerProfile["app"]
	if pc.Exec != 8 || pc.Skip != 0 {
		t.Errorf("app = exec:%d skip:%d, want exec:8 skip:0", pc.Exec, pc.Skip)
	}

	if ad.HasSkipped {
		t.Error("HasSkipped = true, want false")
	}

	aks := ad.ActionsByProfile["app"]
	if len(aks) != 2 {
		t.Fatalf("actions count = %d, want 2", len(aks))
	}
	// Actions sorted alphabetically.
	if aks[0].Action != "alert" || aks[1].Action != "done" {
		t.Errorf("action order = [%s, %s], want [alert, done]", aks[0].Action, aks[1].Action)
	}
}

func TestAggregateGroupsMultipleProfiles(t *testing.T) {
	groups := []DayGroup{
		{
			Date: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC),
			Summaries: []DaySummary{
				{Profile: "boss", Action: "done", Executions: 4, Skipped: 0},
				{Profile: "dev", Action: "done", Executions: 6, Skipped: 0},
			},
		},
		{
			Date: time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC),
			Summaries: []DaySummary{
				{Profile: "boss", Action: "done", Executions: 2, Skipped: 0},
			},
		},
	}

	ad := AggregateGroups(groups)

	if len(ad.ProfileOrder) != 2 || ad.ProfileOrder[0] != "boss" || ad.ProfileOrder[1] != "dev" {
		t.Errorf("ProfileOrder = %v, want [boss dev]", ad.ProfileOrder)
	}

	boss := ad.PerProfile["boss"]
	if boss.Exec != 6 {
		t.Errorf("boss exec = %d, want 6", boss.Exec)
	}
	dev := ad.PerProfile["dev"]
	if dev.Exec != 6 {
		t.Errorf("dev exec = %d, want 6", dev.Exec)
	}
}

func TestAggregateGroupsWithSkipped(t *testing.T) {
	groups := []DayGroup{{
		Date: time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC),
		Summaries: []DaySummary{
			{Profile: "app", Action: "done", Executions: 5, Skipped: 3},
		},
	}}

	ad := AggregateGroups(groups)

	if !ad.HasSkipped {
		t.Error("HasSkipped = false, want true")
	}

	ak := ActionKey{"app", "done"}
	ac := ad.PerAction[ak]
	if ac.Skip != 3 {
		t.Errorf("skip = %d, want 3", ac.Skip)
	}
}

// --- ComputeHourly ---

func TestComputeHourlyEmpty(t *testing.T) {
	hd := ComputeHourly(nil, time.Now(), time.UTC)
	if len(hd.PerCell) != 0 {
		t.Errorf("PerCell len = %d, want 0", len(hd.PerCell))
	}
	if hd.GrandTotal != 0 {
		t.Errorf("GrandTotal = %d, want 0", hd.GrandTotal)
	}
}

func TestComputeHourlySingleHour(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 15, 0, 0, loc), Profile: "app", Action: "done", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 30, 0, 0, loc), Profile: "app", Action: "done", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 45, 0, 0, loc), Profile: "dev", Action: "done", Kind: KindExecution},
	}

	hd := ComputeHourly(entries, target, loc)

	if hd.MinHour != 10 || hd.MaxHour != 10 {
		t.Errorf("hour range = %d-%d, want 10-10", hd.MinHour, hd.MaxHour)
	}
	if hd.GrandTotal != 3 {
		t.Errorf("GrandTotal = %d, want 3", hd.GrandTotal)
	}
	if c := hd.PerCell[HourProfile{10, "app"}]; c != 2 {
		t.Errorf("app@10 = %d, want 2", c)
	}
	if c := hd.PerCell[HourProfile{10, "dev"}]; c != 1 {
		t.Errorf("dev@10 = %d, want 1", c)
	}
}

func TestComputeHourlyMultipleHours(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 8, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 30, 0, 0, loc), Profile: "dev", Action: "x", Kind: KindCooldown},
	}

	hd := ComputeHourly(entries, target, loc)

	if hd.MinHour != 8 || hd.MaxHour != 10 {
		t.Errorf("hour range = %d-%d, want 8-10", hd.MinHour, hd.MaxHour)
	}
	if hd.GrandTotal != 3 {
		t.Errorf("GrandTotal = %d, want 3", hd.GrandTotal)
	}
	if len(hd.Profiles) != 2 {
		t.Errorf("profiles = %d, want 2", len(hd.Profiles))
	}

	// ProfileTotals should be computed.
	appIdx := 0
	if hd.Profiles[0] != "app" {
		appIdx = 1
	}
	if hd.ProfileTotals[appIdx] != 2 {
		t.Errorf("app total = %d, want 2", hd.ProfileTotals[appIdx])
	}
}

func TestComputeHourlyWrongDateFiltered(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 24, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 26, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
	}

	hd := ComputeHourly(entries, target, loc)

	if len(hd.PerCell) != 0 {
		t.Errorf("PerCell len = %d, want 0 (wrong dates should be filtered)", len(hd.PerCell))
	}
}

func TestComputeHourlyKindOtherExcluded(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindOther},
	}

	hd := ComputeHourly(entries, target, loc)

	if len(hd.PerCell) != 0 {
		t.Errorf("PerCell len = %d, want 0 (KindOther should be excluded)", len(hd.PerCell))
	}
}

// --- ComputeTimeSpent ---

func TestComputeTimeSpentEmpty(t *testing.T) {
	td := ComputeTimeSpent(nil, time.Now(), time.UTC)
	if len(td.Profiles) != 0 {
		t.Errorf("Profiles len = %d, want 0", len(td.Profiles))
	}
	if td.Total != 0 {
		t.Errorf("Total = %d, want 0", td.Total)
	}
}

func TestComputeTimeSpentWithinThreshold(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 3, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 5, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	if len(td.Profiles) != 1 {
		t.Fatalf("Profiles len = %d, want 1", len(td.Profiles))
	}
	// 3min + 2min = 5min = 300 seconds
	if td.Profiles[0].Seconds != 300 {
		t.Errorf("app seconds = %d, want 300", td.Profiles[0].Seconds)
	}
	if td.Total != 300 {
		t.Errorf("Total = %d, want 300", td.Total)
	}
}

func TestComputeTimeSpentExceedingThreshold(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 2, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		// 10-minute gap — exceeds 5min threshold, not counted.
		{Time: time.Date(2026, 2, 25, 10, 12, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 14, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	// 2min + (10min gap skipped) + 2min = 4min = 240 seconds
	if td.Profiles[0].Seconds != 240 {
		t.Errorf("app seconds = %d, want 240", td.Profiles[0].Seconds)
	}
}

func TestComputeTimeSpentSingleEntry(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	if len(td.Profiles) != 1 {
		t.Fatalf("Profiles len = %d, want 1", len(td.Profiles))
	}
	// Single entry = 0 seconds.
	if td.Profiles[0].Seconds != 0 {
		t.Errorf("app seconds = %d, want 0", td.Profiles[0].Seconds)
	}
}

func TestComputeTimeSpentMultipleProfiles(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 1, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "dev", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 4, 0, 0, loc), Profile: "dev", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	if len(td.Profiles) != 2 {
		t.Fatalf("Profiles len = %d, want 2", len(td.Profiles))
	}
	// app: 1min=60s, dev: 4min=240s
	// Total uses merged timeline (10:00, 10:00, 10:01, 10:04) = 0+60+180 = 240s
	// (not 300, which would double-count the overlap)
	if td.Total != 240 {
		t.Errorf("Total = %d, want 240", td.Total)
	}
	// Profiles sorted alphabetically.
	if td.Profiles[0].Name != "app" || td.Profiles[0].Seconds != 60 {
		t.Errorf("app = %+v, want {app 60}", td.Profiles[0])
	}
	if td.Profiles[1].Name != "dev" || td.Profiles[1].Seconds != 240 {
		t.Errorf("dev = %+v, want {dev 240}", td.Profiles[1])
	}
}

func TestComputeTimeSpentOverlappingProfilesMerged(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	// Three profiles all active during the same 10:00-10:03 window.
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "a", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 3, 0, 0, loc), Profile: "a", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "b", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 3, 0, 0, loc), Profile: "b", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "c", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 3, 0, 0, loc), Profile: "c", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	// Each profile: 3min = 180s
	for _, p := range td.Profiles {
		if p.Seconds != 180 {
			t.Errorf("%s seconds = %d, want 180", p.Name, p.Seconds)
		}
	}
	// Total should be 180s (3 minutes wall-clock), not 540s (3×180).
	// Merged: 10:00, 10:00, 10:00, 10:03, 10:03, 10:03 → 0+0+180+0+0 = 180s
	if td.Total != 180 {
		t.Errorf("Total = %d, want 180 (merged, not 540)", td.Total)
	}
}

func TestComputeTimeSpentNonOverlappingProfiles(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	// Two profiles active in completely separate time windows (>5min gap between).
	entries := []Entry{
		{Time: time.Date(2026, 2, 25, 10, 0, 0, 0, loc), Profile: "morning", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 10, 2, 0, 0, loc), Profile: "morning", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 14, 0, 0, 0, loc), Profile: "afternoon", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 25, 14, 3, 0, 0, loc), Profile: "afternoon", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	// morning: 2min=120s, afternoon: 3min=180s
	// No overlap, so total = sum = 300s
	if td.Profiles[0].Seconds != 180 { // afternoon (sorted)
		t.Errorf("afternoon seconds = %d, want 180", td.Profiles[0].Seconds)
	}
	if td.Profiles[1].Seconds != 120 { // morning (sorted)
		t.Errorf("morning seconds = %d, want 120", td.Profiles[1].Seconds)
	}
	if td.Total != 300 {
		t.Errorf("Total = %d, want 300 (no overlap)", td.Total)
	}
}

func TestComputeTimeSpentWrongDateFiltered(t *testing.T) {
	loc := time.UTC
	target := time.Date(2026, 2, 25, 0, 0, 0, 0, loc)
	entries := []Entry{
		{Time: time.Date(2026, 2, 24, 10, 0, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
		{Time: time.Date(2026, 2, 24, 10, 1, 0, 0, loc), Profile: "app", Action: "x", Kind: KindExecution},
	}

	td := ComputeTimeSpent(entries, target, loc)

	if len(td.Profiles) != 0 {
		t.Errorf("Profiles len = %d, want 0", len(td.Profiles))
	}
}
