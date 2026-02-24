package eventlog

import (
	"sort"
	"strings"
	"time"
)

// EntryKind classifies a log entry.
type EntryKind int

const (
	KindExecution EntryKind = iota
	KindCooldown
	KindSilent
	KindOther
)

// Entry is a single parsed log entry.
type Entry struct {
	Time    time.Time
	Profile string
	Action  string
	Kind    EntryKind
}

// DaySummary holds execution and skip counts for one profile/action pair.
type DaySummary struct {
	Profile    string
	Action     string
	Executions int
	Skipped    int
}

// DayGroup holds all summaries for a single calendar day.
type DayGroup struct {
	Date      time.Time
	Summaries []DaySummary
}

// ParseEntries splits log content on blank lines and parses summary lines
// into entries. Each block may contain multiple summary lines (e.g. a
// cooldown=recorded line followed by an execution line) plus step detail
// lines (indented with "step["). Step detail lines are skipped. Each
// summary line produces one Entry. Malformed lines are silently skipped.
func ParseEntries(content string) []Entry {
	content = strings.TrimRight(content, "\n\r ")
	if content == "" {
		return nil
	}

	blocks := strings.Split(content, "\n\n")
	var entries []Entry
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		for _, line := range strings.Split(block, "\n") {
			// Skip step detail lines (indented, contain "step[").
			if strings.Contains(line, "step[") {
				continue
			}

			// Timestamp is everything before the first "  " (two spaces).
			tsEnd := strings.Index(line, "  ")
			if tsEnd < 0 {
				continue
			}
			ts, err := time.Parse(time.RFC3339, line[:tsEnd])
			if err != nil {
				continue
			}

			profile := extractField(line, "profile")
			action := extractField(line, "action")

			// Skip lines without profile/action (silent=enabled, silent=disabled, etc.)
			if profile == "" || action == "" {
				continue
			}

			// Classify.
			kind := KindOther
			if hasField(line, "steps") {
				kind = KindExecution
			} else if extractField(line, "cooldown") == "skipped" {
				kind = KindCooldown
			} else if extractField(line, "silent") == "skipped" {
				kind = KindSilent
			}

			entries = append(entries, Entry{
				Time:    ts,
				Profile: profile,
				Action:  action,
				Kind:    kind,
			})
		}
	}
	return entries
}

// SummarizeByDay filters entries to the last N calendar days (local time),
// groups by date + profile/action, and returns day groups sorted descending
// with summaries sorted alphabetically. Pass days=0 to include all entries.
func SummarizeByDay(entries []Entry, days int) []DayGroup {
	now := time.Now()
	var cutoff time.Time
	if days > 0 {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff = today.AddDate(0, 0, -(days - 1))
	}

	// Group by date string + profile/action key.
	type key struct {
		date           string
		profile, action string
	}
	type counts struct {
		executions, skipped int
	}
	grouped := map[key]*counts{}
	dates := map[string]time.Time{}

	for _, e := range entries {
		local := e.Time.In(now.Location())
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, now.Location())
		if days > 0 && day.Before(cutoff) {
			continue
		}

		// Only count executions, cooldown skips, and silent skips.
		if e.Kind == KindOther {
			continue
		}

		ds := day.Format("2006-01-02")
		k := key{date: ds, profile: e.Profile, action: e.Action}
		c, ok := grouped[k]
		if !ok {
			c = &counts{}
			grouped[k] = c
			dates[ds] = day
		}

		if e.Kind == KindExecution {
			c.executions++
		} else {
			c.skipped++
		}
	}

	// Build day groups.
	dayMap := map[string]*DayGroup{}
	for k, c := range grouped {
		dg, ok := dayMap[k.date]
		if !ok {
			dg = &DayGroup{Date: dates[k.date]}
			dayMap[k.date] = dg
		}
		dg.Summaries = append(dg.Summaries, DaySummary{
			Profile:    k.profile,
			Action:     k.action,
			Executions: c.executions,
			Skipped:    c.skipped,
		})
	}

	// Sort summaries alphabetically within each day.
	for _, dg := range dayMap {
		sort.Slice(dg.Summaries, func(i, j int) bool {
			ki := dg.Summaries[i].Profile + "/" + dg.Summaries[i].Action
			kj := dg.Summaries[j].Profile + "/" + dg.Summaries[j].Action
			return ki < kj
		})
	}

	// Collect and sort days descending.
	groups := make([]DayGroup, 0, len(dayMap))
	for _, dg := range dayMap {
		groups = append(groups, *dg)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Date.After(groups[j].Date)
	})

	return groups
}

// extractField returns the value after "key=" in a space-separated line.
// Returns "" if not found.
func extractField(line, key string) string {
	prefix := key + "="
	for _, field := range strings.Fields(line) {
		if strings.HasPrefix(field, prefix) {
			return field[len(prefix):]
		}
	}
	return ""
}

// KindString returns a human-readable string for an EntryKind.
func KindString(k EntryKind) string {
	switch k {
	case KindExecution:
		return "execution"
	case KindCooldown:
		return "cooldown"
	case KindSilent:
		return "silent"
	default:
		return "other"
	}
}

// hasField returns true if "key=" appears in the line.
func hasField(line, key string) bool {
	return extractField(line, key) != ""
}
