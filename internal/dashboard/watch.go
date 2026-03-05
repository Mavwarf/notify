package dashboard

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Mavwarf/notify/internal/eventlog"
)

// Watch-related types used by handleWatch and the frontend.

type watchAction struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Exec    int    `json:"exec"`
	Skipped int    `json:"skipped"`
}

type watchProfile struct {
	Name    string        `json:"name"`
	Total   int           `json:"total"`
	Exec    int           `json:"exec"`
	Skipped int           `json:"skipped"`
	Pct     int           `json:"pct"`
	Actions []watchAction `json:"actions"`
}

type watchSummary struct {
	Profiles     []watchProfile `json:"profiles"`
	GrandTotal   int            `json:"grand_total"`
	GrandExec    int            `json:"grand_exec"`
	GrandSkipped int            `json:"grand_skipped"`
}

type watchBucketRow struct {
	Label  string `json:"label"`
	Counts []int  `json:"counts"`
	Total  int    `json:"total"`
	Pct    int    `json:"pct"`
}

type watchBreakdown struct {
	BucketLabel   string           `json:"bucket_label"`
	Profiles      []string         `json:"profiles"`
	Buckets       []watchBucketRow `json:"buckets"`
	ProfileTotals []int            `json:"profile_totals"`
	GrandTotal    int              `json:"grand_total"`
}

type watchTimeProfile struct {
	Name    string `json:"name"`
	Seconds int    `json:"seconds"`
}

type watchTimeSpent struct {
	Profiles []watchTimeProfile `json:"profiles"`
	Total    int                `json:"total"`
}

type watchResponse struct {
	Date       string         `json:"date"`
	DayName    string         `json:"day_name"`
	Range      string         `json:"range"`
	RangeLabel string         `json:"range_label"`
	IsToday    bool           `json:"is_today"`
	Summary    watchSummary   `json:"summary"`
	Hourly     watchBreakdown `json:"hourly"`
	TimeSpent  watchTimeSpent `json:"time_spent"`
}

// computeRange returns the start and end dates for the given range type
// anchored at the specified date.
func computeRange(anchor time.Time, rangeType string, loc *time.Location) (time.Time, time.Time) {
	y, m, d := anchor.Year(), anchor.Month(), anchor.Day()
	switch rangeType {
	case "week":
		day := time.Date(y, m, d, 0, 0, 0, 0, loc)
		wd := day.Weekday()
		// Monday=0 offset
		offset := int(wd) - int(time.Monday)
		if offset < 0 {
			offset += 7
		}
		start := day.AddDate(0, 0, -offset)
		end := start.AddDate(0, 0, 6)
		return start, end
	case "month":
		start := time.Date(y, m, 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, -1)
		return start, end
	case "year":
		start := time.Date(y, 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(y, 12, 31, 0, 0, 0, 0, loc)
		return start, end
	case "total":
		// Sentinel dates that bracket any realistic event timestamp. Using
		// 2000-01-01 and 2099-12-31 ensures the range includes all stored
		// entries without needing to scan for actual min/max dates first.
		start := time.Date(2000, 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(2099, 12, 31, 0, 0, 0, 0, loc)
		return start, end
	default: // "day"
		day := time.Date(y, m, d, 0, 0, 0, 0, loc)
		return day, day
	}
}

// formatRangeLabel returns a human-readable label for a date range.
func formatRangeLabel(start, end time.Time, rangeType string) string {
	switch rangeType {
	case "week":
		return start.Format("Jan 2") + " – " + end.Format("Jan 2, 2006")
	case "month":
		return start.Format("January 2006")
	case "year":
		return start.Format("2006")
	case "total":
		return "All time"
	default: // "day"
		return start.Format("2006-01-02") + "  (" + start.Format("Monday") + ")"
	}
}

// computeBreakdown builds a generalized bucket breakdown for the given entries
// and date range. For "day" range it buckets by hour, for "week"/"month" by day,
// and for "year"/"total" by month.
func computeBreakdown(entries []eventlog.Entry, start, end time.Time, rangeType string, loc *time.Location) watchBreakdown {
	bd := watchBreakdown{
		Profiles: []string{},
		Buckets:  []watchBucketRow{},
	}

	switch rangeType {
	case "year", "total":
		bd.BucketLabel = "Month"
	case "week", "month":
		bd.BucketLabel = "Day"
	default:
		bd.BucketLabel = "Hour"
	}

	type bucketKey struct {
		idx     int
		profile string
	}
	perCell := map[bucketKey]int{}
	perBucket := map[int]int{}
	profileSet := map[string]bool{}
	// Sentinel values: start at max-int / min-int so the first real bucket
	// index always replaces them. Used by "day" and "total" range types to
	// trim the output to only the buckets that contain data.
	minIdx, maxIdx := 1<<31, -(1 << 31)

	// bucketIndex maps an entry time to its bucket index.
	bucketIndex := func(t time.Time) int {
		local := t.In(loc)
		switch rangeType {
		case "year", "total":
			// Month-based: months since start year-month
			return (local.Year()-start.Year())*12 + int(local.Month()) - int(start.Month())
		case "week", "month":
			// Day-based: calendar days since start. Using math.Round avoids
			// off-by-one errors on DST transition days (23h or 25h spans).
			localDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
			return int(math.Round(localDay.Sub(start).Hours() / 24))
		default:
			return local.Hour()
		}
	}

	// Filter entries to range and bucket them.
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, loc)

	for _, e := range entries {
		local := e.Time.In(loc)
		if local.Before(startDay) || local.After(endDay) || e.Kind == eventlog.KindOther {
			continue
		}
		idx := bucketIndex(e.Time)
		perCell[bucketKey{idx, e.Profile}]++
		perBucket[idx]++
		profileSet[e.Profile] = true
		if idx < minIdx {
			minIdx = idx
		}
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	if len(perCell) == 0 {
		return bd
	}

	profiles := make([]string, 0, len(profileSet))
	for p := range profileSet {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)
	bd.Profiles = profiles

	// For "day" range, trim to min..max hours (same as original behavior).
	// For other ranges, show all expected buckets.
	var startIdx, endIdx int
	switch rangeType {
	case "day":
		startIdx = minIdx
		endIdx = maxIdx
	case "week":
		startIdx = 0
		dur := end.Sub(start)
		endIdx = int(dur.Hours() / 24)
	case "month":
		startIdx = 0
		endIdx = end.Day() - 1
	case "year":
		startIdx = 0
		endIdx = 11
	case "total":
		startIdx = minIdx
		endIdx = maxIdx
	}

	// bucketLabel generates a human-readable label for a bucket index.
	// For month-based ranges, bucket 0 corresponds to start.Month(). Adding
	// the index gives the absolute month, then we convert back to
	// year + Month via divmod-12 arithmetic (1-based, so subtract/add 1).
	bucketLabel := func(idx int) string {
		switch rangeType {
		case "year":
			// Within a single year: start.Month()+idx gives Jan..Dec directly.
			m := time.Month(int(start.Month()) + idx)
			return m.String()[:3]
		case "total":
			// Across multiple years: compute absolute month offset from the
			// start date, then split into year and 1-based month.
			base := int(start.Month()) + idx
			y := start.Year() + (base-1)/12
			m := time.Month(((base - 1) % 12) + 1)
			return fmt.Sprintf("%s '%02d", m.String()[:3], y%100)
		case "week":
			d := start.AddDate(0, 0, idx)
			return d.Format("Mon 2")
		case "month":
			return fmt.Sprintf("%d", idx+1)
		default: // day
			return fmt.Sprintf("%02d:00", idx)
		}
	}

	grandTotal := 0
	for _, c := range perBucket {
		grandTotal += c
	}
	bd.GrandTotal = grandTotal

	buckets := make([]watchBucketRow, 0, endIdx-startIdx+1)
	profileTotals := make([]int, len(profiles))
	for i := startIdx; i <= endIdx; i++ {
		cnts := make([]int, len(profiles))
		for j, p := range profiles {
			c := perCell[bucketKey{i, p}]
			cnts[j] = c
			profileTotals[j] += c
		}
		bt := perBucket[i]
		pct := 0
		if grandTotal > 0 {
			pct = bt * 100 / grandTotal
		}
		buckets = append(buckets, watchBucketRow{
			Label:  bucketLabel(i),
			Counts: cnts,
			Total:  bt,
			Pct:    pct,
		})
	}
	bd.Buckets = buckets
	bd.ProfileTotals = profileTotals
	return bd
}

// computeTimeSpentRange estimates approximate time spent per profile across
// all entries within [start, end]. Same 5-min-gap algorithm as ComputeTimeSpent.
func computeTimeSpentRange(entries []eventlog.Entry, start, end time.Time, loc *time.Location) watchTimeSpent {
	gapThreshold := eventlog.GapThreshold

	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, loc)

	profileEntries := map[string][]time.Time{}
	for _, e := range entries {
		local := e.Time.In(loc)
		if local.Before(startDay) || local.After(endDay) || e.Kind == eventlog.KindOther {
			continue
		}
		profileEntries[e.Profile] = append(profileEntries[e.Profile], e.Time)
	}

	if len(profileEntries) == 0 {
		return watchTimeSpent{Profiles: []watchTimeProfile{}}
	}

	names := make([]string, 0, len(profileEntries))
	for p := range profileEntries {
		names = append(names, p)
	}
	sort.Strings(names)

	var tps []watchTimeProfile
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
		tps = append(tps, watchTimeProfile{Name: p, Seconds: secs})
		allTimes = append(allTimes, times...)
	}

	sort.Slice(allTimes, func(i, j int) bool { return allTimes[i].Before(allTimes[j]) })
	total := 0
	for i := 1; i < len(allTimes); i++ {
		gap := allTimes[i].Sub(allTimes[i-1])
		if gap <= gapThreshold {
			total += int(gap.Seconds())
		}
	}

	return watchTimeSpent{Profiles: tps, Total: total}
}
