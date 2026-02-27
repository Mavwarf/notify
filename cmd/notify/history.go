package main

import (
	"encoding/json"
	"fmt"
	"os"
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
		case "remove":
			historyRemove(args[1:])
			return
		}
	}

	count := 10
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fatal("count must be a positive integer")
		}
		count = n
	}

	data, ok := readLog()
	if !ok {
		fmt.Println("No log file found. Enable logging with --log or \"log\": true in config.")
		return
	}

	content := strings.TrimRight(data, "\n\r ")
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
				fatal("days must be a positive integer or \"all\"")
			}
			days = n
		}
	}

	data, ok := readLog()
	if !ok {
		fmt.Println("No log file found. Enable logging with --log or \"log\": true in config.")
		return
	}

	entries := eventlog.ParseEntries(data)
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
func renderTableRows(w *strings.Builder, ad eventlog.AggregatedData, baseline map[string]int, hasNew bool, grandTotal int) int {
	totalNew := 0

	for pi, profile := range ad.ProfileOrder {
		if pi > 0 {
			w.WriteString("\n")
		}
		aks := ad.ActionsByProfile[profile]
		pc := ad.PerProfile[profile]
		pTotal := pc.Exec + pc.Skip

		// Profile subtotal row.
		w.WriteString("  " + padR(cyan(profile), colProfile+(len(cyan(profile))-len(profile))))
		w.WriteString(" " + padL(fmtNum(pTotal), colNumber))
		w.WriteString("  " + padL(fmtPct(pTotal, grandTotal), colPct))
		if ad.HasSkipped {
			if pc.Skip > 0 {
				w.WriteString("  " + colorPadL(yellow, fmtNum(pc.Skip), colNumber))
			} else {
				w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
			}
		}
		if hasNew {
			pNew := 0
			for _, ak := range aks {
				key := ak.Profile + "/" + ak.Action
				c := ad.PerAction[ak]
				pNew += (c.Exec + c.Skip) - baseline[key]
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
			c := ad.PerAction[ak]
			aTotal := c.Exec + c.Skip
			fmt.Fprintf(w, "    %-*s %*s", colAction, ak.Action, colNumber, fmtNum(aTotal))
			w.WriteString(fmt.Sprintf("  %*s", colPct, ""))
			if ad.HasSkipped {
				if c.Skip > 0 {
					w.WriteString("  " + colorPadL(yellow, fmtNum(c.Skip), colNumber))
				} else {
					w.WriteString(fmt.Sprintf("  %*s", colNumber, ""))
				}
			}
			if hasNew {
				key := ak.Profile + "/" + ak.Action
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
func renderTableTotal(w *strings.Builder, ad eventlog.AggregatedData, hasNew bool, totalNew int, sep string) {
	w.WriteString(sep + "\n")

	grandExec := 0
	grandSkip := 0
	for _, pc := range ad.PerProfile {
		grandExec += pc.Exec
		grandSkip += pc.Skip
	}
	grandTotal := grandExec + grandSkip
	totalLine := fmt.Sprintf("  %-*s %*s  %*s", colProfile, "Total", colNumber, fmtNum(grandTotal), colPct, "")

	if ad.HasSkipped {
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
	ad := eventlog.AggregateGroups(groups)
	hasNew := baseline != nil

	grandTotal := 0
	for _, pc := range ad.PerProfile {
		grandTotal += pc.Exec + pc.Skip
	}

	sep := dim("  " + strings.Repeat("─", sepBase+sepPerCol*btoi(ad.HasSkipped)+sepPerCol*btoi(hasNew)))

	renderTableHeader(w, groups, ad.HasSkipped, hasNew, sep)
	totalNew := renderTableRows(w, ad, baseline, hasNew, grandTotal)
	renderTableTotal(w, ad, hasNew, totalNew, sep)
}

// renderHourlyTable writes a per-hour activity breakdown.
// Columns: one per profile + a Total column, rows: one per hour from first
// activity to the last activity hour.
func renderHourlyTable(w *strings.Builder, entries []eventlog.Entry) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	hd := eventlog.ComputeHourly(entries, today, now.Location())
	if len(hd.PerCell) == 0 {
		return
	}

	// Column widths: at least colNumber, or the profile name length.
	colWidths := make([]int, len(hd.Profiles))
	for i, p := range hd.Profiles {
		pw := len(p)
		if pw < colNumber {
			pw = colNumber
		}
		colWidths[i] = pw
	}

	const colHr = 7  // "HH:00" + padding
	const colTot = 7 // "Total"

	// Separator width.
	sepW := colHr
	for _, cw := range colWidths {
		sepW += colGap + cw
	}
	sepW += colGap + colTot + colGap + colPct

	w.WriteString("\n")

	// Header.
	hdr := bold(fmt.Sprintf("  %-*s", colHr, "Hour"))
	for i, p := range hd.Profiles {
		hdr += "  " + colorPadL(cyan, p, colWidths[i])
	}
	hdr += bold(fmt.Sprintf("  %*s  %*s", colTot, "Total", colPct, "%"))
	w.WriteString(hdr + "\n")

	sep := dim("  " + strings.Repeat("─", sepW))
	w.WriteString(sep + "\n")

	// Data rows.
	for h := hd.MinHour; h <= hd.MaxHour; h++ {
		row := fmt.Sprintf("  %-*s", colHr, fmt.Sprintf("%02d:00", h))
		for i, p := range hd.Profiles {
			c := hd.PerCell[eventlog.HourProfile{Hour: h, Profile: p}]
			if c > 0 {
				row += "  " + padL(fmtNum(c), colWidths[i])
			} else {
				row += "  " + colorPadL(dim, "-", colWidths[i])
			}
		}
		ht := hd.PerHour[h]
		if ht > 0 {
			row += "  " + padL(fmtNum(ht), colTot)
			row += "  " + padL(fmtPct(ht, hd.GrandTotal), colPct)
		} else {
			row += "  " + colorPadL(dim, "-", colTot)
			row += fmt.Sprintf("  %*s", colPct, "")
		}
		w.WriteString(row + "\n")
	}

	// Total row.
	w.WriteString(sep + "\n")
	totRow := fmt.Sprintf("  %-*s", colHr, "Total")
	for i := range hd.Profiles {
		totRow += "  " + padL(fmtNum(hd.ProfileTotals[i]), colWidths[i])
	}
	totRow += fmt.Sprintf("  %*s  %*s", colTot, fmtNum(hd.GrandTotal), colPct, "")
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
		fatal("%v", err)
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
		fatal("days must be a positive integer")
	}

	data, ok := readLog()
	if !ok {
		fmt.Println("Log file is empty.")
		return
	}
	path := eventlog.LogPath()

	content := strings.TrimRight(data, "\n\r ")
	if content == "" {
		fmt.Println("Log file is empty.")
		return
	}

	// Count original non-empty blocks for the "removed" message.
	origBlocks := 0
	for _, b := range strings.Split(content, "\n\n") {
		if strings.TrimSpace(b) != "" {
			origBlocks++
		}
	}

	filtered := eventlog.FilterBlocksByDays(content, days)

	keptBlocks := 0
	if filtered != "" {
		for _, b := range strings.Split(filtered, "\n\n") {
			if strings.TrimSpace(b) != "" {
				keptBlocks++
			}
		}
	}
	removed := origBlocks - keptBlocks

	if filtered == "" {
		_ = os.Remove(path)
		fmt.Printf("Removed %d entries. Log file cleared.\n", removed)
		return
	}

	out := filtered + "\n\n"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("Removed %d entries, kept %d (last %d days).\n", removed, keptBlocks, days)
}

func historyRemove(args []string) {
	if len(args) == 0 {
		fatal("Usage: notify history remove <profile>")
	}
	profileName := args[0]

	data, ok := readLog()
	if !ok {
		fmt.Println("Log file is empty.")
		return
	}
	path := eventlog.LogPath()

	content := strings.TrimRight(data, "\n\r ")
	if content == "" {
		fmt.Println("Log file is empty.")
		return
	}

	filtered, removed := eventlog.FilterBlocksByProfile(content, profileName)
	if removed == 0 {
		fmt.Printf("No entries found for profile %q.\n", profileName)
		return
	}

	if filtered == "" {
		_ = os.Remove(path)
		fmt.Printf("Removed %d entries for profile %q. Log file cleared.\n", removed, profileName)
		return
	}

	out := filtered + "\n\n"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("Removed %d entries for profile %q.\n", removed, profileName)
}

func historyWatch() {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fatal("cannot enter raw mode: %v", err)
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
		fmt.Fprintf(&out, "notify history watch  —  started %s (%s)\n%s\n\n",
			started.Format("15:04:05"), dim(elapsed.String()),
			dim("press x or Esc to exit"))

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
			if key == 'x' || key == 'X' || key == 3 || key == 27 { // x, X, Ctrl+C, or Esc
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
			fatal("days must be a positive integer")
		}
		days = n
	}

	data, ok := readLog()
	if !ok {
		fmt.Println("[]")
		return
	}

	entries := eventlog.ParseEntries(data)

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
