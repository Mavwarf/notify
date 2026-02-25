package eventlog

import (
	"sort"
	"strconv"
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

			ts, ok := ExtractTimestamp(line)
			if !ok {
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

// ExtractTimestamp parses the RFC3339 timestamp at the start of a log line
// (everything before the first "  " double-space separator). Returns the
// parsed time and true on success, or zero time and false on failure.
func ExtractTimestamp(line string) (time.Time, bool) {
	tsEnd := strings.Index(line, "  ")
	if tsEnd < 0 {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, line[:tsEnd])
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
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

// extractQuoted extracts a Go %q-encoded string from the start of s.
// It finds the matching closing quote (respecting backslash escapes),
// then uses strconv.Unquote to decode the value. Returns "" on failure.
func extractQuoted(s string) string {
	if len(s) == 0 || s[0] != '"' {
		return ""
	}
	// Find closing quote (skip escaped quotes).
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped character
			continue
		}
		if s[i] == '"' {
			text, err := strconv.Unquote(s[:i+1])
			if err != nil {
				return ""
			}
			return text
		}
	}
	return ""
}

// VoiceLine holds a unique say-step text and its usage count.
type VoiceLine struct {
	Text  string
	Count int
}

// ParseVoiceLines scans log content for say step detail lines and returns
// unique texts sorted by frequency (descending), then alphabetically.
func ParseVoiceLines(content string) []VoiceLine {
	content = strings.TrimRight(content, "\n\r ")
	if content == "" {
		return nil
	}

	counts := map[string]int{}
	for _, line := range strings.Split(content, "\n") {
		// Match step detail lines for say steps: "step[N] say  text="..."
		idx := strings.Index(line, "] say  text=")
		if idx < 0 {
			continue
		}
		if !strings.Contains(line[:idx], "step[") {
			continue
		}

		// Extract the quoted text value. The text is %q-encoded and may be
		// followed by additional fields (e.g. when=afk, volume=80).
		raw := line[idx+len("] say  text="):]
		text := extractQuoted(raw)
		if text != "" {
			counts[text]++
		}
	}

	if len(counts) == 0 {
		return nil
	}

	lines := make([]VoiceLine, 0, len(counts))
	for text, count := range counts {
		lines = append(lines, VoiceLine{Text: text, Count: count})
	}

	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Count != lines[j].Count {
			return lines[i].Count > lines[j].Count
		}
		return lines[i].Text < lines[j].Text
	})

	return lines
}
