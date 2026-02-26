package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/voice"
)

const defaultMinUses = 3

func voiceCmd(args []string, configPath string) {
	if len(args) > 0 {
		switch args[0] {
		case "stats":
			voiceStats(args[1:])
			return
		case "generate":
			voiceGenerate(args[1:], configPath)
			return
		case "list":
			voiceList()
			return
		case "play":
			voicePlay(args[1:])
			return
		case "test":
			voiceTest(args[1:], configPath)
			return
		case "clear":
			voiceClear()
			return
		}
	}
	fmt.Fprintln(os.Stderr, `Usage: notify voice <command>

Commands:
  generate [--min-uses N]                    Generate AI voice files for frequently used say steps
  test [--voice V] [--speed S] [--model M] <text>  Generate and play a voice line on the fly
  play [text]                                Play all cached voices, or one matching text
  list                                       List cached voice files
  clear                                      Delete all cached voice files
  stats [days|all]                           Show say step text usage frequency`)
	os.Exit(1)
}

func voiceGenerate(args []string, configPath string) {
	minUses := -1 // -1 = not set by flag, use config or default

	// Parse flags.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--min-uses":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "Error: --min-uses requires a non-negative integer\n")
					os.Exit(1)
				}
				minUses = n
				i++
			}
		}
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Resolve voice settings.
	voiceName := cfg.Options.Voice.Voice
	if voiceName == "" {
		voiceName = "nova"
	}
	model := cfg.Options.Voice.Model
	if model == "" {
		model = "tts-1"
	}
	speed := cfg.Options.Voice.Speed
	if speed == 0 {
		speed = 1.0
	}

	// Resolve min uses threshold.
	threshold := defaultMinUses
	if minUses >= 0 {
		threshold = minUses
	} else if cfg.Options.Voice.MinUses > 0 {
		threshold = cfg.Options.Voice.MinUses
	}

	// Get API key.
	apiKey := cfg.Options.Credentials.OpenAIAPIKey
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: openai_api_key not configured\n")
		fmt.Fprintf(os.Stderr, "Add to config: \"credentials\": { \"openai_api_key\": \"$OPENAI_API_KEY\" }\n")
		os.Exit(1)
	}

	// Read voice line usage from event log.
	logData, err := os.ReadFile(eventlog.LogPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No log file found. Enable logging with --log or \"log\": true to track usage.")
			fmt.Println("Voice generation requires usage data to identify frequently used messages.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error reading log: %v\n", err)
		os.Exit(1)
	}

	voiceLines := eventlog.ParseVoiceLines(string(logData))
	if len(voiceLines) == 0 {
		fmt.Println("No say step usage found in the event log.")
		return
	}

	// Build usage count map: text -> count.
	usageCounts := make(map[string]int, len(voiceLines))
	for _, vl := range voiceLines {
		usageCounts[vl.Text] = vl.Count
	}

	// Open voice cache.
	cache, err := voice.OpenCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening voice cache: %v\n", err)
		os.Exit(1)
	}

	// Filter candidates: frequently used, not dynamic, not already cached.
	var toGenerate []struct {
		text  string
		count int
	}
	var skippedCached int
	var skippedDynamic int
	var skippedBelowThreshold int

	for text, count := range usageCounts {
		if count < threshold {
			skippedBelowThreshold++
			continue
		}
		if tmpl.HasDynamic(text) {
			skippedDynamic++
			continue
		}
		if _, ok := cache.Lookup(text); ok {
			skippedCached++
			continue
		}
		toGenerate = append(toGenerate, struct {
			text  string
			count int
		}{text, count})
	}

	// Sort by usage count descending for priority.
	sort.Slice(toGenerate, func(i, j int) bool {
		return toGenerate[i].count > toGenerate[j].count
	})

	if len(toGenerate) == 0 {
		fmt.Printf("Nothing to generate (threshold: %d uses)\n", threshold)
		if skippedCached > 0 {
			fmt.Printf("  %d already cached\n", skippedCached)
		}
		if skippedDynamic > 0 {
			fmt.Printf("  %d skipped (dynamic variables)\n", skippedDynamic)
		}
		if skippedBelowThreshold > 0 {
			fmt.Printf("  %d below threshold (%d uses)\n", skippedBelowThreshold, threshold)
		}
		return
	}

	fmt.Printf("Generating %d voice files (model: %s, voice: %s, threshold: %d uses)...\n\n",
		len(toGenerate), model, voiceName, threshold)

	var generated, failed int
	for i, item := range toGenerate {
		if i > 0 {
			// Pace requests to stay within OpenAI rate limits (free tier: 3 RPM).
			fmt.Printf("  (waiting 21s to avoid rate limit...)\n")
			time.Sleep(21 * time.Second)
		}
		fmt.Printf("  [%d/%d] %q (%d uses)... ", i+1, len(toGenerate), item.text, item.count)
		wavData, err := voice.Generate(apiKey, model, voiceName, item.text, speed)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			failed++
			continue
		}
		if err := cache.Add(item.text, voiceName, wavData); err != nil {
			fmt.Printf("FAILED (save): %v\n", err)
			failed++
			continue
		}
		fmt.Printf("OK (%d KB)\n", len(wavData)/1024)
		generated++
	}

	fmt.Printf("\nDone: %d generated", generated)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	if skippedCached > 0 {
		fmt.Printf(", %d already cached", skippedCached)
	}
	if skippedDynamic > 0 {
		fmt.Printf(", %d dynamic (system tts)", skippedDynamic)
	}
	if skippedBelowThreshold > 0 {
		fmt.Printf(", %d below threshold", skippedBelowThreshold)
	}
	fmt.Println()
}

func voiceList() {
	cache, err := voice.OpenCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(cache.Entries) == 0 {
		fmt.Println("Voice cache is empty. Run 'notify voice generate' to create AI voice files.")
		return
	}

	// Sort entries by creation time (newest first).
	type entry struct {
		hash string
		e    voice.CacheEntry
	}
	entries := make([]entry, 0, len(cache.Entries))
	for h, e := range cache.Entries {
		entries = append(entries, entry{h, e})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].e.CreatedAt.After(entries[j].e.CreatedAt)
	})

	var totalSize int64
	for _, e := range entries {
		totalSize += e.e.Size
	}

	fmt.Printf("Voice cache: %d files, %s total\n\n", len(entries), formatSize(totalSize))

	hdr := fmt.Sprintf("  %-18s  %-8s  %-7s  %-20s  %s", "Hash", "Voice", "Size", "Created", "Text")
	fmt.Println(bold(hdr))
	fmt.Println(dim("  " + strings.Repeat("─", 80)))

	for _, e := range entries {
		fmt.Printf("  %-18s  %-8s  %-7s  %-20s  %s\n",
			e.hash,
			e.e.Voice,
			formatSize(e.e.Size),
			e.e.CreatedAt.Format("2006-01-02 15:04"),
			dim("\"")+e.e.Text+dim("\""))
	}
}

func voiceTest(args []string, configPath string) {
	// Defaults from config, overridable by flags.
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	voiceName := cfg.Options.Voice.Voice
	if voiceName == "" {
		voiceName = "nova"
	}
	model := cfg.Options.Voice.Model
	if model == "" {
		model = "tts-1"
	}
	speed := cfg.Options.Voice.Speed
	if speed == 0 {
		speed = 1.0
	}

	apiKey := cfg.Options.Credentials.OpenAIAPIKey

	// Parse flags.
	var textParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--voice":
			if i+1 < len(args) {
				voiceName = args[i+1]
				i++
			}
		case "--speed":
			if i+1 < len(args) {
				v, err := strconv.ParseFloat(args[i+1], 64)
				if err != nil || v < 0.25 || v > 4.0 {
					fmt.Fprintf(os.Stderr, "Error: --speed must be 0.25-4.0\n")
					os.Exit(1)
				}
				speed = v
				i++
			}
		case "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		default:
			textParts = append(textParts, args[i])
		}
	}

	text := strings.Join(textParts, " ")
	if text == "" {
		fmt.Fprintf(os.Stderr, "Usage: notify voice test [--voice V] [--speed S] [--model M] <text>\n")
		fmt.Fprintf(os.Stderr, "\nVoices: alloy, echo, fable, onyx, nova, shimmer\n")
		fmt.Fprintf(os.Stderr, "Models: tts-1, tts-1-hd\n")
		fmt.Fprintf(os.Stderr, "Speed:  0.25-4.0 (default: 1.0)\n")
		os.Exit(1)
	}

	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: openai_api_key not configured\n")
		os.Exit(1)
	}

	fmt.Printf("Generating: %q (voice: %s, model: %s, speed: %.1f)...\n", text, voiceName, model, speed)

	wavData, err := voice.Generate(apiKey, model, voiceName, text, speed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Playing (%d KB)...\n", len(wavData)/1024)

	// Write to temp file and play.
	tmpFile, err := os.CreateTemp("", "notify-voice-test-*.wav")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(wavData); err != nil {
		tmpFile.Close()
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	if err := audio.Play(tmpPath, 1.0); err != nil {
		fmt.Fprintf(os.Stderr, "Error playing: %v\n", err)
		os.Exit(1)
	}
}

func voiceClear() {
	cache, err := voice.OpenCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	count, err := cache.Clear()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if count == 0 {
		fmt.Println("Voice cache was already empty.")
	} else {
		fmt.Printf("Cleared %d cached voice files.\n", count)
	}
}

func voicePlay(args []string) {
	cache, err := voice.OpenCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(cache.Entries) == 0 {
		fmt.Println("Voice cache is empty. Run 'notify voice generate' to create AI voice files.")
		return
	}

	// Collect and sort entries by text for consistent order.
	type entry struct {
		hash string
		e    voice.CacheEntry
	}
	entries := make([]entry, 0, len(cache.Entries))
	for h, e := range cache.Entries {
		entries = append(entries, entry{h, e})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].e.Text < entries[j].e.Text
	})

	// If a text argument is given, filter to matching entries.
	if len(args) > 0 {
		search := strings.Join(args, " ")
		var matched []entry
		for _, e := range entries {
			if strings.EqualFold(e.e.Text, search) || strings.Contains(strings.ToLower(e.e.Text), strings.ToLower(search)) {
				matched = append(matched, e)
			}
		}
		if len(matched) == 0 {
			fmt.Fprintf(os.Stderr, "No cached voice matching %q\n", search)
			os.Exit(1)
		}
		entries = matched
	}

	for i, e := range entries {
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		wavPath, ok := cache.Lookup(e.e.Text)
		if !ok {
			fmt.Fprintf(os.Stderr, "  %q — WAV file missing, skipping\n", e.e.Text)
			continue
		}
		fmt.Printf("  [%d/%d] %q\n", i+1, len(entries), e.e.Text)
		if err := audio.Play(wavPath, 1.0); err != nil {
			fmt.Fprintf(os.Stderr, "    Error: %v\n", err)
		}
	}
}

// formatSize returns a human-readable file size string.
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.0f KB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.1f MB", mb)
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
		content = eventlog.FilterBlocksByDays(content, days)
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
