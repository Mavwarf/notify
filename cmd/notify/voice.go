package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/eventlog"
)

func voiceCmd(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "stats":
			voiceStats(args[1:])
			return
		}
	}
	fmt.Fprintln(os.Stderr, "Usage: notify voice stats [days|all]")
	os.Exit(1)
}

func voiceStats(args []string) {
	days := 0 // default: all time
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

	content := string(data)

	// Filter to date range if days specified.
	if days > 0 {
		content = filterContentByDays(content, days)
	}

	lines := eventlog.ParseVoiceLines(content)
	if len(lines) == 0 {
		if days > 0 {
			fmt.Printf("No voice lines found in the last %d days.\n", days)
		} else {
			fmt.Println("No voice lines found.")
		}
		return
	}

	var out strings.Builder
	renderVoiceTable(&out, lines, days)
	fmt.Print(out.String())
}

// filterContentByDays returns only log blocks whose timestamp falls within
// the last N calendar days.
func filterContentByDays(content string, days int) string {
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
	return strings.Join(kept, "\n\n")
}

// renderVoiceTable writes a formatted table of voice line statistics.
func renderVoiceTable(w *strings.Builder, lines []eventlog.VoiceLine, days int) {
	total := 0
	for _, l := range lines {
		total += l.Count
	}

	// Header.
	if days > 0 {
		fmt.Fprintf(w, "Voice line statistics (last %d days, %s total)\n", days, fmtNum(total))
	} else {
		fmt.Fprintf(w, "Voice line statistics (all time, %s total)\n", fmtNum(total))
	}
	w.WriteString("\n")

	const colRank = 3
	const colCount = 7

	hdr := fmt.Sprintf("  %*s  %*s  %*s  %s", colRank, "#", colCount, "Count", colPct, "%", "Text")
	w.WriteString(bold(hdr) + "\n")

	sep := dim("  " + strings.Repeat("─", colRank+2+colCount+2+colPct+2+30))
	w.WriteString(sep + "\n")

	// Data rows.
	for i, l := range lines {
		fmt.Fprintf(w, "  %*d  %*s  %*s  %s\n",
			colRank, i+1,
			colCount, fmtNum(l.Count),
			colPct, fmtPct(l.Count, total),
			dim("\"")+l.Text+dim("\""))
	}
}
