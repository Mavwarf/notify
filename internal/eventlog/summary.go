package eventlog

import (
	"sort"
	"strings"
	"time"
)

// SplitBlocks splits log content on blank lines, trims whitespace from
// each block, and returns only non-empty blocks.
func SplitBlocks(content string) []string {
	raw := strings.Split(content, "\n\n")
	blocks := make([]string, 0, len(raw))
	for _, b := range raw {
		b = strings.TrimSpace(b)
		if b != "" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// DayCutoff returns midnight N days ago (inclusive) in the local timezone.
// For days=1 it returns today at midnight, for days=7 it returns 6 days ago, etc.
func DayCutoff(days int) time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.AddDate(0, 0, -(days - 1))
}

// ActionKey identifies a profile/action pair for aggregation.
type ActionKey struct{ Profile, Action string }

// Counts holds execution and skip totals for a profile or action.
type Counts struct{ Exec, Skip int }

// AggregatedData holds the result of aggregating day groups into
// per-action and per-profile counts.
type AggregatedData struct {
	PerAction        map[ActionKey]*Counts
	PerProfile       map[string]*Counts
	ProfileOrder     []string
	ActionsByProfile map[string][]ActionKey
	HasSkipped       bool
}

// HourProfile is the key for per-cell hourly data.
type HourProfile struct {
	Hour    int
	Profile string
}

// HourlyData holds computed hourly breakdown data for a single day.
type HourlyData struct {
	Profiles      []string
	PerCell       map[HourProfile]int
	PerHour       map[int]int
	ProfileTotals []int // parallel to Profiles
	MinHour       int
	MaxHour       int
	GrandTotal    int
}

// TimeSpentProfile holds approximate time spent for one profile.
type TimeSpentProfile struct {
	Name    string
	Seconds int
}

// TimeSpentData holds approximate time spent across all profiles.
type TimeSpentData struct {
	Profiles []TimeSpentProfile
	Total    int
}

// AggregateGroups collects per-action and per-profile counts from day groups.
func AggregateGroups(groups []DayGroup) AggregatedData {
	ad := AggregatedData{
		PerAction:        map[ActionKey]*Counts{},
		PerProfile:       map[string]*Counts{},
		ActionsByProfile: map[string][]ActionKey{},
	}
	profileSeen := map[string]bool{}

	for _, dg := range groups {
		for _, s := range dg.Summaries {
			ak := ActionKey{s.Profile, s.Action}
			ac, ok := ad.PerAction[ak]
			if !ok {
				ac = &Counts{}
				ad.PerAction[ak] = ac
			}
			ac.Exec += s.Executions
			ac.Skip += s.Skipped

			pc, ok := ad.PerProfile[s.Profile]
			if !ok {
				pc = &Counts{}
				ad.PerProfile[s.Profile] = pc
			}
			pc.Exec += s.Executions
			pc.Skip += s.Skipped

			if !profileSeen[s.Profile] {
				profileSeen[s.Profile] = true
				ad.ProfileOrder = append(ad.ProfileOrder, s.Profile)
			}
		}
	}
	sort.Strings(ad.ProfileOrder)

	for ak := range ad.PerAction {
		ad.ActionsByProfile[ak.Profile] = append(ad.ActionsByProfile[ak.Profile], ak)
		if ak.Profile != "" && ad.PerAction[ak].Skip > 0 {
			ad.HasSkipped = true
		}
	}
	for _, aks := range ad.ActionsByProfile {
		sort.Slice(aks, func(i, j int) bool { return aks[i].Action < aks[j].Action })
	}
	if !ad.HasSkipped {
		for _, c := range ad.PerAction {
			if c.Skip > 0 {
				ad.HasSkipped = true
				break
			}
		}
	}

	return ad
}

// ComputeHourly computes per-hour activity breakdown for a target date.
// Only entries matching targetDate in the given location are included.
// Entries with KindOther are excluded.
func ComputeHourly(entries []Entry, targetDate time.Time, loc *time.Location) HourlyData {
	hd := HourlyData{
		PerCell: map[HourProfile]int{},
		PerHour: map[int]int{},
		MinHour: 24,
		MaxHour: -1,
	}
	profileSet := map[string]bool{}

	for _, e := range entries {
		local := e.Time.In(loc)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		if !day.Equal(targetDate) || e.Kind == KindOther {
			continue
		}
		h := local.Hour()
		hd.PerCell[HourProfile{h, e.Profile}]++
		hd.PerHour[h]++
		profileSet[e.Profile] = true
		if h < hd.MinHour {
			hd.MinHour = h
		}
		if h > hd.MaxHour {
			hd.MaxHour = h
		}
	}

	if len(hd.PerCell) == 0 {
		return hd
	}

	hd.Profiles = make([]string, 0, len(profileSet))
	for p := range profileSet {
		hd.Profiles = append(hd.Profiles, p)
	}
	sort.Strings(hd.Profiles)

	for _, c := range hd.PerHour {
		hd.GrandTotal += c
	}

	hd.ProfileTotals = make([]int, len(hd.Profiles))
	for h := hd.MinHour; h <= hd.MaxHour; h++ {
		for i, p := range hd.Profiles {
			hd.ProfileTotals[i] += hd.PerCell[HourProfile{h, p}]
		}
	}

	return hd
}

// ComputeTimeSpent estimates approximate time spent per profile on a target
// date. It walks consecutive entry timestamps per profile; gaps of 5 minutes
// or less are counted as active time.
func ComputeTimeSpent(entries []Entry, targetDate time.Time, loc *time.Location) TimeSpentData {
	const gapThreshold = 5 * time.Minute

	profileEntries := map[string][]time.Time{}
	for _, e := range entries {
		local := e.Time.In(loc)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		if !day.Equal(targetDate) || e.Kind == KindOther {
			continue
		}
		profileEntries[e.Profile] = append(profileEntries[e.Profile], e.Time)
	}

	if len(profileEntries) == 0 {
		return TimeSpentData{}
	}

	names := make([]string, 0, len(profileEntries))
	for p := range profileEntries {
		names = append(names, p)
	}
	sort.Strings(names)

	// Compute per-profile time.
	var td TimeSpentData
	var allTimes []time.Time
	for _, p := range names {
		times := profileEntries[p]
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		secs := 0
		for i := 1; i < len(times); i++ {
			gap := times[i].Sub(times[i-1])
			if gap <= gapThreshold {
				secs += int(gap.Seconds())
			}
		}
		td.Profiles = append(td.Profiles, TimeSpentProfile{Name: p, Seconds: secs})
		allTimes = append(allTimes, times...)
	}

	// Compute total from merged timeline so overlapping profiles don't inflate.
	sort.Slice(allTimes, func(i, j int) bool { return allTimes[i].Before(allTimes[j]) })
	for i := 1; i < len(allTimes); i++ {
		gap := allTimes[i].Sub(allTimes[i-1])
		if gap <= gapThreshold {
			td.Total += int(gap.Seconds())
		}
	}

	return td
}

// FilterBlocksByProfile removes all log blocks belonging to the named profile.
// Returns the filtered content and the number of removed blocks.
func FilterBlocksByProfile(content string, profile string) (string, int) {
	blocks := SplitBlocks(content)
	var kept []string
	removed := 0
	for _, block := range blocks {
		firstLine := block
		if idx := strings.Index(block, "\n"); idx > 0 {
			firstLine = block[:idx]
		}
		if extractField(firstLine, "profile") == profile {
			removed++
		} else {
			kept = append(kept, block)
		}
	}
	return strings.Join(kept, "\n\n"), removed
}

// FilterBlocksByDays returns only log blocks whose timestamp falls within
// the last N calendar days. Each block is separated by a blank line.
func FilterBlocksByDays(content string, days int) string {
	cutoff := DayCutoff(days)

	var kept []string
	for _, block := range SplitBlocks(content) {
		firstLine := block
		if idx := strings.Index(block, "\n"); idx > 0 {
			firstLine = block[:idx]
		}
		ts, ok := ExtractTimestamp(firstLine)
		if !ok {
			continue
		}
		if !ts.In(cutoff.Location()).Before(cutoff) {
			kept = append(kept, block)
		}
	}
	return strings.Join(kept, "\n\n")
}
