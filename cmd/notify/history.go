package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Mavwarf/notify/internal/eventlog"
)

func historyCmd(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "summary":
			historySummary(args[1:])
			return
		case "clear":
			historyClear()
			return
		case "clean":
			historyClean(args[1:])
			return
		case "export":
			historyExport(args[1:])
			return
		case "watch":
			historyWatch()
			return
		}
	}

	count := 10
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "Error: count must be a positive integer\n")
			os.Exit(1)
		}
		count = n
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No log file found. Enable logging with --log or \"log\": true in config.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		fmt.Println("Log file is empty.")
		return
	}

	entries := strings.Split(content, "\n\n")
	if len(entries) > count {
		entries = entries[len(entries)-count:]
	}
	for i, e := range entries {
		fmt.Print(e)
		fmt.Println()
		if i < len(entries)-1 {
			fmt.Println()
		}
	}
}

func historySummary(args []string) {
	days := 7
	if len(args) > 0 {
		if args[0] == "all" {
			days = 0
		} else {
			n, err := strconv.Atoi(args[0])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "Error: days must be a positive integer or \"all\"\n")
				os.Exit(1)
			}
			days = n
		}
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No log file found. Enable logging with --log or \"log\": true in config.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	entries := eventlog.ParseEntries(string(data))
	groups := eventlog.SummarizeByDay(entries, days)

	if len(groups) == 0 {
		if days == 0 {
			fmt.Println("No activity found.")
		} else {
			fmt.Println("No activity in the last", days, "days.")
		}
		return
	}

	var out strings.Builder
	renderSummaryTable(&out, groups, nil)
	fmt.Print(out.String())
}

// --- Table layout constants ---

const (
	colProfile = 24 // width of profile name column
	colAction  = 22 // width of action name column (indented by 2)
	colNumber  = 7  // width of numeric columns (Total, Skipped, New)
	colGap     = 2  // gap between numeric columns
	colPct     = 5  // width of percentage column (fits " 100%")
	// Base separator width covers the fixed columns: profile, Total, and %.
	sepBase       = colProfile + colNumber + colGap + 1 + colGap + colPct // 40
	sepPerCol     = colGap + colNumber                                    // 9
	watchInterval = 2 * time.Second
)

// --- ANSI color helpers (disabled when NO_COLOR env var is set) ---

var noColor = os.Getenv("NO_COLOR") != ""

func ansi(code, s string) string {
	if noColor {
		return s
	}
	return code + s + "\033[0m"
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func bold(s string) string   { return ansi("\033[1m", s) }
func dim(s string) string    { return ansi("\033[2m", s) }
func cyan(s string) string   { return ansi("\033[36m", s) }
func green(s string) string  { return ansi("\033[32m", s) }
func yellow(s string) string { return ansi("\033[33m", s) }

// fmtNum formats an integer with dot as thousands separator (e.g. 1234 → "1.234").
func fmtNum(n int) string {
	neg := ""
	if n < 0 {
		neg = "-"
		n = -n
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return neg + s
	}
	var buf strings.Builder
	r := len(s) % 3
	if r > 0 {
		buf.WriteString(s[:r])
	}
	for i := r; i < len(s); i += 3 {
		if buf.Len() > 0 {
			buf.WriteByte('.')
		}
		buf.WriteString(s[i : i+3])
	}
	return neg + buf.String()
}

// fmtPct formats n as a percentage of total (e.g. "68%"), or "" if total is 0.
func fmtPct(n, total int) string {
	if total == 0 {
		return ""
	}
	return strconv.Itoa(n*100/total) + "%"
}

// padL pads s to width with spaces on the left.
func padL(s string, width int) string {
	if pad := width - len(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// padR pads s to width with spaces on the right.
func padR(s string, width int) string {
	if pad := width - len(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// colorPadL applies a color function to s, then left-pads to width
// (accounting for invisible ANSI escape bytes).
func colorPadL(colorFn func(string) string, s string, width int) string {
	colored := colorFn(s)
	return padL(colored, width+(len(colored)-len(s)))
}

// --- Summary table types ---

type actionKey struct{ profile, action string }
type counts struct{ exec, skip int }

type tableData struct {
	perAction        map[actionKey]*counts
	perProfile       map[string]*counts
	profileOrder     []string
	actionsByProfile map[string][]actionKey
	hasSkipped       bool
}

// aggregateGroups collects per-action and per-profile counts from day groups.
func aggregateGroups(groups []eventlog.DayGroup) tableData {
	td := tableData{
		perAction:        map[actionKey]*counts{},
		perProfile:       map[string]*counts{},
		actionsByProfile: map[string][]actionKey{},
	}
	profileSeen := map[string]bool{}

	for _, dg := range groups {
		for _, s := range dg.Summaries {
			ak := actionKey{s.Profile, s.Action}
			ac, ok := td.perAction[ak]
			if !ok {
				ac = &counts{}
				td.perAction[ak] = ac
			}
			ac.exec += s.Executions
			ac.skip += s.Skipped

			pc, ok := td.perProfile[s.Profile]
			if !ok {
				pc = &counts{}
				td.perProfile[s.Profile] = pc
			}
			pc.exec += s.Executions
			pc.skip += s.Skipped

			if !profileSeen[s.Profile] {
				profileSeen[s.Profile] = true
				td.profileOrder = append(td.profileOrder, s.Profile)
			}
		}
	}
	sort.Strings(td.profileOrder)

	for ak := range td.perAction {
		td.actionsByProfile[ak.profile] = append(td.actionsByProfile[ak.profile], ak)
		if ak.profile != "" && td.perAction[ak].skip > 0 {
			td.hasSkipped = true
		}
	}
	for _, aks := range td.actionsByProfile {
		sort.Slice(aks, func(i, j int) bool { return aks[i].action < aks[j].action })
	}
	if !td.hasSkipped {
		for _, c := range td.perAction {
			if c.skip > 0 {
				td.hasSkipped = true
				break
			}
		}
	}

	return td
}

// renderTableHeader writes the date line, column header, and separator.
func renderTableHeader(w *strings.Builder, groups []eventlog.DayGroup, hasSkipped, hasNew bool, sep string) {
	if len(groups) == 1 {
		dg := groups[0]
		fmt.Fprintf(w, "%s\n", dim(fmt.Sprintf("%s  (%s)", dg.Date.Format("2006-01-02"), dg.Date.Format("Monday"))))
	} else {
		fmt.Fprintf(w, "%s\n", dim(fmt.Sprintf("%s — %s",
			groups[0].Date.Format("2006-01-02"),
			groups[len(groups)-1].Date.Format("2006-01-02"))))
	}

	hdr := fmt.Sprintf("  %-*s %*s  %*s", colProfile, "", colNumber, "Total", colPct, "%")
	if hasSkipped {
		hdr += fmt.Sprintf("  %*s", colNumber, "Skipped")
	}
	if hasNew {
		hdr += fmt.Sprintf("  %*s", colNumber, "New")
	}
	w.WriteString(bold(hdr) + "\n")
	w.WriteString(sep + "\n")
}

// renderTableRows writes profile subtotal and per-action rows.
// Returns the total "new" count across all profiles.
func renderTableRows(w *strings.Builder, td tableData, baseline map[string]int, hasNew bool, grandTotal int) int {
	totalNew := 0

	for pi, profile := range td.profileOrder {
		if pi > 0 {
			w.WriteString("\n")
		}
		aks := td.actionsByProfile[profile]
		pc := td.perProfile[profile]
		pTotal := pc.exec + pc.skip

		// Profile subtotal row.
		w.WriteString("  " + padR(cyan(profile), colProfile+(len(cyan(profile))-len(profile))))
		w.WriteString(" " + padL(fmtNum(pTotal), colNumber))
		w.WriteString("  " + padL(fmtPct(pTotal, grandTotal), colPct))
		if td.hasSkipped {
			if pc.skip > 0 {
				w.WriteString("  " + colorPadL(yellow, fmtNum(pc.skip), colNumber))
			} else {
				w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
			}
		}
		if hasNew {
			pNew := 0
			for _, ak := range aks {
				key := ak.profile + "/" + ak.action
				c := td.perAction[ak]
				pNew += (c.exec + c.skip) - baseline[key]
			}
			if pNew > 0 {
				w.WriteString("  " + colorPadL(green, "+"+fmtNum(pNew), colNumber))
			} else {
				w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
			}
			totalNew += pNew
		}
		w.WriteString("\n")

		// Action rows (indented).
		for _, ak := range aks {
			c := td.perAction[ak]
			aTotal := c.exec + c.skip
			fmt.Fprintf(w, "    %-*s %*s", colAction, ak.action, colNumber, fmtNum(aTotal))
			w.WriteString(fmt.Sprintf("  %*s", colPct, ""))
			if td.hasSkipped {
				if c.skip > 0 {
					w.WriteString("  " + colorPadL(yellow, fmtNum(c.skip), colNumber))
				} else {
					w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
				}
			}
			if hasNew {
				key := ak.profile + "/" + ak.action
				aN := aTotal - baseline[key]
				if aN > 0 {
					w.WriteString("  " + colorPadL(green, "+"+fmtNum(aN), colNumber))
				} else {
					w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
				}
			}
			w.WriteString("\n")
		}
	}
	return totalNew
}

// renderTableTotal writes the separator and bold total row.
func renderTableTotal(w *strings.Builder, td tableData, hasNew bool, totalNew int, sep string) {
	w.WriteString(sep + "\n")

	grandExec := 0
	grandSkip := 0
	for _, pc := range td.perProfile {
		grandExec += pc.exec
		grandSkip += pc.skip
	}
	grandTotal := grandExec + grandSkip
	totalLine := fmt.Sprintf("  %-*s %*s  %*s", colProfile, "Total", colNumber, fmtNum(grandTotal), colPct, "")

	if td.hasSkipped {
		if grandSkip > 0 {
			w.WriteString(bold(totalLine))
			w.WriteString("  " + colorPadL(yellow, fmtNum(grandSkip), colNumber))
			totalLine = ""
		} else {
			totalLine += fmt.Sprintf("  %*s", colNumber, "")
		}
	}
	if hasNew && totalLine != "" {
		w.WriteString(bold(totalLine))
		if totalNew > 0 {
			w.WriteString("  " + colorPadL(green, "+"+fmtNum(totalNew), colNumber))
		} else {
			w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
		}
	} else if totalLine != "" {
		w.WriteString(bold(totalLine))
	} else if hasNew {
		if totalNew > 0 {
			w.WriteString("  " + colorPadL(green, "+"+fmtNum(totalNew), colNumber))
		} else {
			w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
		}
	}
	w.WriteString("\n")
}

// renderSummaryTable writes a formatted table of notification stats.
// When baseline is non-nil (watch mode), a "New" column shows deltas.
func renderSummaryTable(w *strings.Builder, groups []eventlog.DayGroup, baseline map[string]int) {
	td := aggregateGroups(groups)
	hasNew := baseline != nil

	grandTotal := 0
	for _, pc := range td.perProfile {
		grandTotal += pc.exec + pc.skip
	}

	sep := dim("  " + strings.Repeat("─", sepBase+sepPerCol*btoi(td.hasSkipped)+sepPerCol*btoi(hasNew)))

	renderTableHeader(w, groups, td.hasSkipped, hasNew, sep)
	totalNew := renderTableRows(w, td, baseline, hasNew, grandTotal)
	renderTableTotal(w, td, hasNew, totalNew, sep)
}

// renderHourlyTable writes a per-hour activity breakdown.
// Columns: one per profile + a Total column, rows: one per hour from first
// activity to the current hour.
func renderHourlyTable(w *strings.Builder, entries []eventlog.Entry) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	type hp struct {
		hour    int
		profile string
	}
	perCell := map[hp]int{}
	perHour := map[int]int{}
	profileSet := map[string]bool{}
	minHour, maxHour := 24, -1

	for _, e := range entries {
		local := e.Time.In(now.Location())
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, now.Location())
		if !day.Equal(today) || e.Kind == eventlog.KindOther {
			continue
		}
		h := local.Hour()
		perCell[hp{h, e.Profile}]++
		perHour[h]++
		profileSet[e.Profile] = true
		if h < minHour {
			minHour = h
		}
		if h > maxHour {
			maxHour = h
		}
	}

	if len(perCell) == 0 {
		return
	}

	profiles := make([]string, 0, len(profileSet))
	for p := range profileSet {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)

	// Extend to current hour so quiet periods are visible.
	if curH := now.Hour(); curH > maxHour {
		maxHour = curH
	}

	// Column widths: at least colNumber, or the profile name length.
	colWidths := make([]int, len(profiles))
	for i, p := range profiles {
		pw := len(p)
		if pw < colNumber {
			pw = colNumber
		}
		colWidths[i] = pw
	}

	const colHr = 7  // "HH:00" + padding
	const colTot = 7 // "Total"

	// Pre-compute grand total for percentage calculation.
	grandTotal := 0
	for _, c := range perHour {
		grandTotal += c
	}

	// Separator width.
	sepW := colHr
	for _, cw := range colWidths {
		sepW += colGap + cw
	}
	sepW += colGap + colTot + colGap + colPct

	w.WriteString("\n")

	// Header.
	hdr := bold(fmt.Sprintf("  %-*s", colHr, "Hour"))
	for i, p := range profiles {
		hdr += "  " + colorPadL(cyan, p, colWidths[i])
	}
	hdr += bold(fmt.Sprintf("  %*s  %*s", colTot, "Total", colPct, "%"))
	w.WriteString(hdr + "\n")

	sep := dim("  " + strings.Repeat("─", sepW))
	w.WriteString(sep + "\n")

	// Data rows.
	grandPerProfile := make([]int, len(profiles))

	for h := minHour; h <= maxHour; h++ {
		row := fmt.Sprintf("  %-*s", colHr, fmt.Sprintf("%02d:00", h))
		for i, p := range profiles {
			c := perCell[hp{h, p}]
			grandPerProfile[i] += c
			if c > 0 {
				row += "  " + padL(fmtNum(c), colWidths[i])
			} else {
				row += "  " + colorPadL(dim, "-", colWidths[i])
			}
		}
		ht := perHour[h]
		if ht > 0 {
			row += "  " + padL(fmtNum(ht), colTot)
			row += "  " + padL(fmtPct(ht, grandTotal), colPct)
		} else {
			row += "  " + colorPadL(dim, "-", colTot)
			row += fmt.Sprintf("  %*s", colPct, "")
		}
		w.WriteString(row + "\n")
	}

	// Total row.
	w.WriteString(sep + "\n")
	totRow := fmt.Sprintf("  %-*s", colHr, "Total")
	for i := range profiles {
		totRow += "  " + padL(fmtNum(grandPerProfile[i]), colWidths[i])
	}
	totRow += fmt.Sprintf("  %*s  %*s", colTot, fmtNum(grandTotal), colPct, "")
	w.WriteString(bold(totRow) + "\n")
}

// buildBaseline snapshots current per-action totals for watch delta tracking.
func buildBaseline(groups []eventlog.DayGroup) map[string]int {
	b := map[string]int{}
	for _, dg := range groups {
		for _, s := range dg.Summaries {
			b[s.Profile+"/"+s.Action] += s.Executions + s.Skipped
		}
	}
	return b
}

func historyClear() {
	path := eventlog.LogPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Log file cleared.")
}

func historyClean(args []string) {
	if len(args) == 0 {
		// No days argument — clear everything.
		historyClear()
		return
	}

	days, err := strconv.Atoi(args[0])
	if err != nil || days <= 0 {
		fmt.Fprintf(os.Stderr, "Error: days must be a positive integer\n")
		os.Exit(1)
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Log file is empty.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		fmt.Println("Log file is empty.")
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, -(days - 1))

	blocks := strings.Split(content, "\n\n")
	var kept []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		firstLine := block
		if idx := strings.Index(block, "\n"); idx > 0 {
			firstLine = block[:idx]
		}
		ts, ok := eventlog.ExtractTimestamp(firstLine)
		if !ok {
			continue
		}
		if !ts.In(now.Location()).Before(cutoff) {
			kept = append(kept, block)
		}
	}

	removed := len(blocks) - len(kept)

	if len(kept) == 0 {
		_ = os.Remove(path)
		fmt.Printf("Removed %d entries. Log file cleared.\n", removed)
		return
	}

	out := strings.Join(kept, "\n\n") + "\n\n"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed %d entries, kept %d (last %d days).\n", removed, len(kept), days)
}

func historyWatch() {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot enter raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(fd, oldState)

	keys := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				keys <- buf[0]
			}
			if err != nil {
				return
			}
		}
	}()

	var baseline map[string]int
	started := time.Now()
	for {
		elapsed := time.Since(started).Truncate(time.Second)
		var out strings.Builder
		out.WriteString("\033[2J\033[H")
		fmt.Fprintf(&out, "notify history watch  —  started %s (%s)  —  press x to exit\n\n",
			started.Format("15:04:05"), dim(elapsed.String()))

		path := eventlog.LogPath()
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				out.WriteString("No log file found.\n")
			} else {
				fmt.Fprintf(&out, "Error: %v\n", err)
			}
		} else {
			entries := eventlog.ParseEntries(string(data))
			groups := eventlog.SummarizeByDay(entries, 1)
			if len(groups) == 0 {
				out.WriteString("No activity today.\n")
			} else {
				// Capture baseline on first render.
				if baseline == nil {
					baseline = buildBaseline(groups)
				}
				renderSummaryTable(&out, groups, baseline)
				renderHourlyTable(&out, entries)
			}
		}

		// In raw mode \n doesn't include \r, so convert.
		os.Stdout.WriteString(strings.ReplaceAll(out.String(), "\n", "\r\n"))

		timer := time.NewTimer(watchInterval)
		select {
		case key := <-keys:
			timer.Stop()
			if key == 'x' || key == 'X' || key == 3 { // x, X, or Ctrl+C
				os.Stdout.WriteString("\033[2J\033[H")
				return
			}
		case <-timer.C:
		}
	}
}

func historyExport(args []string) {
	days := 0
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "Error: days must be a positive integer\n")
			os.Exit(1)
		}
		days = n
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("[]")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	entries := eventlog.ParseEntries(string(data))

	if days > 0 {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff := today.AddDate(0, 0, -(days - 1))
		var filtered []eventlog.Entry
		for _, e := range entries {
			if !e.Time.In(now.Location()).Before(cutoff) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	type exportEntry struct {
		Time    string `json:"time"`
		Profile string `json:"profile"`
		Action  string `json:"action"`
		Kind    string `json:"kind"`
	}
	out := make([]exportEntry, len(entries))
	for i, e := range entries {
		out[i] = exportEntry{
			Time:    e.Time.Format(time.RFC3339),
			Profile: e.Profile,
			Action:  e.Action,
			Kind:    eventlog.KindString(e.Kind),
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
