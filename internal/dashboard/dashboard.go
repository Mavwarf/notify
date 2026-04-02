// Package dashboard serves the web-based notification management UI.
package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/cooldown"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/idle"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/silent"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/voice"
)

//go:embed static/index.html
var staticFS embed.FS

// Version is the build version string, set by the caller before Serve().
var Version = "dev"

// BuildDate is the build timestamp string (e.g. "2026-03-05T14:30:00Z"),
// set by the caller before Serve().
var BuildDate = "unknown"

// JSON response types used by API handlers.

type jsonEntry struct {
	Time          string `json:"time"`
	Profile       string `json:"profile"`
	Action        string `json:"action"`
	Kind          string `json:"kind"`
	ClaudeHook    string `json:"claude_hook,omitempty"`
	ClaudeMessage string `json:"claude_message,omitempty"`
}

func entryToJSON(e eventlog.Entry) jsonEntry {
	return jsonEntry{
		Time:          e.Time.Format(time.RFC3339),
		Profile:       e.Profile,
		Action:        e.Action,
		Kind:          eventlog.KindString(e.Kind),
		ClaudeHook:    e.ClaudeHook,
		ClaudeMessage: e.ClaudeMessage,
	}
}

type jsonSummary struct {
	Profile    string `json:"profile"`
	Action     string `json:"action"`
	Executions int    `json:"executions"`
	Skipped    int    `json:"skipped"`
}

type jsonGroup struct {
	Date      string        `json:"date"`
	Summaries []jsonSummary `json:"summaries"`
}

type stepResult struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Detail   string `json:"detail"`
	WouldRun bool   `json:"would_run"`
}

type actionResult struct {
	Action    string       `json:"action"`
	Resolved  string       `json:"resolved,omitempty"`
	Steps     []stepResult `json:"steps"`
	TotalRun  int          `json:"total_run"`
	TotalSkip int          `json:"total_skip"`
}

type logStats struct {
	Storage     string `json:"storage"`
	FileSize    int64  `json:"file_size"`
	Entries     int    `json:"entries"`
	OldestEntry string `json:"oldest_entry"`
	NewestEntry string `json:"newest_entry"`
}

type credStatus struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type profileCreds struct {
	Profile     string       `json:"profile"`
	Credentials []credStatus `json:"credentials"`
}

type voiceLine struct {
	Rank   int    `json:"rank"`
	Text   string `json:"text"`
	Count  int    `json:"count"`
	Pct    int    `json:"pct"`
	Cached bool   `json:"cached"`
	Hash   string `json:"hash,omitempty"`
	Voice  string `json:"voice,omitempty"`
}

type voiceResponse struct {
	Lines []voiceLine `json:"lines"`
	Total int         `json:"total"`
}

// Serve starts the dashboard HTTP server on 127.0.0.1:port and blocks until
// interrupted. If open is true, a browser window is launched in app mode
// (chromeless) pointing at the dashboard URL. The showFn, minimizeFn, quitFn,
// and topmostFn callbacks control the native window; the CLI passes nil for all
// of them, while the Wails desktop app supplies real implementations.
func Serve(cfg config.Config, configPath string, port int, open bool, showFn, minimizeFn, quitFn func(), topmostFn func(bool)) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/config", handleConfig(configPath, cfg))
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/summary", handleSummary)
	mux.HandleFunc("/api/events", handleEvents)
	mux.HandleFunc("/api/test", handleTest(configPath, cfg))
	mux.HandleFunc("/api/credentials", handleCredentials(configPath, cfg))
	mux.HandleFunc("/api/watch", handleWatch)
	mux.HandleFunc("/api/stats", handleStats)
	mux.HandleFunc("/api/voice", handleVoice)
	mux.HandleFunc("/api/voice/play/", handleVoicePlay)
	mux.HandleFunc("/api/silent", handleSilent)
	mux.HandleFunc("/api/trigger", handleTrigger(configPath, cfg))
	mux.HandleFunc("/api/preferences", handlePreferences(configPath))
	mux.HandleFunc("/api/edit-config", handleEditConfig(configPath))

	// App-mode-only endpoints: registered only when the corresponding callback
	// is non-nil. The CLI passes nil for all callbacks, so these routes simply
	// don't exist in CLI mode. The frontend detects app mode by probing
	// /api/minimize — a 405 (Method Not Allowed on GET) means app mode, while
	// a 404 (route absent) means CLI mode.
	if showFn != nil {
		mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			showFn()
			w.WriteHeader(http.StatusNoContent)
		})
	}
	if minimizeFn != nil {
		mux.HandleFunc("/api/minimize", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			minimizeFn()
			w.WriteHeader(http.StatusNoContent)
		})
	}
	if quitFn != nil {
		mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			go quitFn()
		})
	}
	if showFn != nil {
		mux.HandleFunc("/api/open", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			rawURL := r.FormValue("url")
			if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
				http.Error(w, "url must start with http:// or https://", http.StatusBadRequest)
				return
			}
			go openSystemBrowser(rawURL)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	if topmostFn != nil {
		mux.HandleFunc("/api/topmost", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				OnTop bool `json:"on_top"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			topmostFn(req.OnTop)
			w.WriteHeader(http.StatusNoContent)
		})
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	url := fmt.Sprintf("http://%s", addr)
	if showFn == nil { // CLI mode — print info for the terminal
		fmt.Printf("Dashboard: %s\n", url)
		fmt.Println("Press Ctrl+C to stop")
	}

	if open {
		go openBrowser(url)
	}

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// openBrowser tries to open the URL in a chromeless browser window (app mode).
// It tries Edge, then Chrome, then falls back to the OS default browser.
func openBrowser(url string) {
	// Browsers that support --app mode (chromeless window).
	appBrowsers := [][]string{
		{"msedge", "--app=" + url},
		{"chrome", "--app=" + url},
		{"google-chrome", "--app=" + url},
		{"chromium", "--app=" + url},
		{"chromium-browser", "--app=" + url},
	}

	for _, b := range appBrowsers {
		if path, err := exec.LookPath(b[0]); err == nil {
			cmd := exec.Command(path, b[1:]...)
			cmd.Stdout = nil
			cmd.Stderr = nil
			if cmd.Start() == nil {
				return
			}
		}
	}

	// Fallback: open in default browser (with address bar).
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// openSystemBrowser opens a URL in the OS default browser.
func openSystemBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// handlePreferences returns the resolved config file path.
func handlePreferences(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := config.FindPath(configPath)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]string{"config_path": p})
	}
}

// handleEditConfig opens the config file in the system's default editor.
func handleEditConfig(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p, err := config.FindPath(configPath)
		if err != nil {
			http.Error(w, "no config file found", http.StatusNotFound)
			return
		}
		go openSystemBrowser(p)
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	html := strings.Replace(string(data), "{{version}}", template.HTMLEscapeString(Version), 1)
	html = strings.Replace(html, "{{buildDate}}", template.HTMLEscapeString(BuildDate), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// loadCfg reloads the config from disk; on any error it returns fallback.
// Called on every request by handlers that need the current config (handleConfig,
// handleTest, handleCredentials, handleTrigger) so that external edits to the
// config file (e.g. from a text editor) are picked up without restarting the server.
func loadCfg(configPath string, fallback config.Config) config.Config {
	if configPath == "" {
		return fallback
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fallback
	}
	return cfg
}

// handleConfig returns the current config as JSON with credential values redacted.
func handleConfig(configPath string, fallback config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadCfg(configPath, fallback)
		redacted := redactConfig(cfg)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(redacted)
	}
}

// handleHistory returns event log entries as JSON, filtered by ?hours or ?days (default 7).
func handleHistory(w http.ResponseWriter, r *http.Request) {
	var entries []eventlog.Entry
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			cutoff := time.Now().Add(-time.Duration(v) * time.Hour)
			entries, _ = eventlog.EntriesSince(cutoff)
		} else {
			entries, _ = eventlog.Entries(7)
		}
	} else {
		days := 7
		if d := r.URL.Query().Get("days"); d != "" {
			if v, err := strconv.Atoi(d); err == nil && v >= 0 {
				days = v
			}
		}
		entries, _ = eventlog.Entries(days)
	}

	out := make([]jsonEntry, len(entries))
	for i, e := range entries {
		out[i] = entryToJSON(e)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(out)
}

// handleSummary returns per-day execution/skip counts grouped by profile and action.
func handleSummary(w http.ResponseWriter, r *http.Request) {
	var entries []eventlog.Entry
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			cutoff := time.Now().Add(-time.Duration(v) * time.Hour)
			entries, _ = eventlog.EntriesSince(cutoff)
		} else {
			entries, _ = eventlog.Entries(0)
		}
	} else {
		entries, _ = eventlog.Entries(0)
	}

	days := 0 // show all loaded entries
	if r.URL.Query().Get("hours") == "" {
		days = 7
		if d := r.URL.Query().Get("days"); d != "" {
			if v, err := strconv.Atoi(d); err == nil && v >= 0 {
				days = v
			}
		}
	}
	groups := eventlog.SummarizeByDay(entries, days)

	out := make([]jsonGroup, len(groups))
	for i, g := range groups {
		sums := make([]jsonSummary, len(g.Summaries))
		for j, s := range g.Summaries {
			sums[j] = jsonSummary{
				Profile:    s.Profile,
				Action:     s.Action,
				Executions: s.Executions,
				Skipped:    s.Skipped,
			}
		}
		out[i] = jsonGroup{
			Date:      g.Date.Format("2006-01-02"),
			Summaries: sums,
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(out)
}

// handleEvents streams new event log entries to the browser via Server-Sent Events.
// It polls the store every 2 seconds and sends only entries added since the last check.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Flush headers immediately so the browser fires onopen.
	flusher.Flush()

	// Snapshot current entry count so we only send new ones.
	// Works for both FileStore and SQLiteStore via the Store interface.
	initial, _ := eventlog.Entries(0)
	seen := len(initial)

	const sseInterval = 2 * time.Second
	ticker := time.NewTicker(sseInterval)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			all, _ := eventlog.Entries(0)

			if len(all) < seen {
				// Clear detection: if the total entry count dropped, the user
				// ran "history clear" or "history clean". Reset the watermark
				// so the next poll cycle picks up from the new baseline rather
				// than sending stale indices. The frontend relies on receiving
				// no data during this tick to trigger a full refresh.
				seen = len(all)
				continue
			}

			if len(all) == seen {
				continue
			}

			newEntries := all[seen:]
			seen = len(all)

			out := make([]jsonEntry, len(newEntries))
			for i, e := range newEntries {
				out[i] = entryToJSON(e)
			}

			jsonData, err := json.Marshal(out)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

// handleTest performs a dry-run of a profile's notification pipeline and returns
// which steps would run or be skipped, with expanded template details.
func handleTest(configPath string, fallback config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadCfg(configPath, fallback)
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Profile string `json:"profile"`
			Action  string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Profile == "" {
			req.Profile = "default"
		}

		// Collect actions to test. Use Resolve() so unknown profiles
		// fall back to "default", matching real CLI behavior.
		var actions []string
		if req.Action != "" {
			actions = []string{req.Action}
		} else {
			// Gather actions from direct profile, then fill in from default.
			seen := map[string]bool{}
			if p, ok := cfg.Profiles[req.Profile]; ok {
				for name := range p.Actions {
					actions = append(actions, name)
					seen[name] = true
				}
			}
			if req.Profile != "default" {
				if dp, ok := cfg.Profiles["default"]; ok {
					for name := range dp.Actions {
						if !seen[name] {
							actions = append(actions, name)
						}
					}
				}
			}
			if len(actions) == 0 {
				http.Error(w, fmt.Sprintf("no actions found for profile %q or default", req.Profile), http.StatusNotFound)
				return
			}
			sort.Strings(actions)
		}

		// Build template vars so {profile}, {Profile}, etc. expand in step details.
		host, _ := os.Hostname()
		now := time.Now()
		vars := tmpl.Vars{
			Profile:  req.Profile,
			Time:     now.Format("15:04"),
			TimeSay:  now.Format("3:04 PM"),
			Date:     now.Format("2006-01-02"),
			DateSay:  now.Format("January 2, 2006"),
			Hostname: host,
		}

		var results []actionResult
		for _, aName := range actions {
			_, act, err := config.Resolve(cfg, req.Profile, aName)
			if err != nil {
				continue
			}

			wouldRun := runner.FilteredIndices(act.Steps, false, false, 0)
			steps := make([]stepResult, len(act.Steps))
			run, skip := 0, 0
			for i, s := range act.Steps {
				detail := eventlog.StepSummary(s, &vars)
				wr := wouldRun[i]
				steps[i] = stepResult{
					Index:    i + 1,
					Type:     s.Type,
					Detail:   detail,
					WouldRun: wr,
				}
				if wr {
					run++
				} else {
					skip++
				}
			}
			ar := actionResult{
				Action:    aName,
				Steps:     steps,
				TotalRun:  run,
				TotalSkip: skip,
			}
			// Show where the action resolved from when it's not a direct hit.
			if p, ok := cfg.Profiles[req.Profile]; !ok || p.Actions == nil {
				ar.Resolved = "default"
			} else if _, ok := p.Actions[aName]; !ok {
				ar.Resolved = "default"
			}
			results = append(results, ar)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(results)
	}
}

// computeRange returns the start and end dates (inclusive) for a given range type
// anchored to a specific date.
// handleWatch returns the Summary, Breakdown, and Time Spent tabs payload: summary counts, time breakdown, and
// estimated time spent, scoped to a date range (?date=YYYY-MM-DD&range=day|week|month|year|total).
func handleWatch(w http.ResponseWriter, r *http.Request) {
	entries, _ := eventlog.Entries(0)

	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Allow browsing any day via ?date=YYYY-MM-DD.
	anchor := today
	if d := r.URL.Query().Get("date"); d != "" {
		if t, err := time.ParseInLocation("2006-01-02", d, loc); err == nil {
			anchor = t
		}
	}

	rangeType := r.URL.Query().Get("range")
	if rangeType == "" {
		rangeType = "day"
	}

	start, end := computeRange(anchor, rangeType, loc)

	// Determine if the range includes today.
	isToday := !today.Before(start) && !today.After(end)

	resp := watchResponse{
		Date:       anchor.Format("2006-01-02"),
		DayName:    anchor.Format("Monday"),
		Range:      rangeType,
		RangeLabel: formatRangeLabel(start, end, rangeType),
		IsToday:    isToday,
		Summary:    watchSummary{Profiles: []watchProfile{}},
		Hourly:     watchBreakdown{Profiles: []string{}, Buckets: []watchBucketRow{}, ProfileTotals: []int{}},
		TimeSpent:  watchTimeSpent{Profiles: []watchTimeProfile{}},
	}

	// Filter day groups to the range.
	groups := eventlog.SummarizeByDay(entries, 0)
	var filteredGroups []eventlog.DayGroup
	for _, g := range groups {
		gDay := time.Date(g.Date.Year(), g.Date.Month(), g.Date.Day(), 0, 0, 0, 0, loc)
		if !gDay.Before(start) && !gDay.After(end) {
			filteredGroups = append(filteredGroups, g)
		}
	}
	groups = filteredGroups

	// Build summary using shared aggregation.
	if len(groups) > 0 {
		ad := eventlog.AggregateGroups(groups)

		grandExec, grandSkip := 0, 0
		for _, pc := range ad.PerProfile {
			grandExec += pc.Exec
			grandSkip += pc.Skip
		}
		grandTotal := grandExec + grandSkip

		profiles := make([]watchProfile, 0, len(ad.ProfileOrder))
		for _, pName := range ad.ProfileOrder {
			pc := ad.PerProfile[pName]
			pTotal := pc.Exec + pc.Skip
			pct := 0
			if grandTotal > 0 {
				pct = pTotal * 100 / grandTotal
			}

			actions := make([]watchAction, 0, len(ad.ActionsByProfile[pName]))
			for _, ak := range ad.ActionsByProfile[pName] {
				ac := ad.PerAction[ak]
				actions = append(actions, watchAction{
					Name:    ak.Action,
					Total:   ac.Exec + ac.Skip,
					Exec:    ac.Exec,
					Skipped: ac.Skip,
				})
			}

			profiles = append(profiles, watchProfile{
				Name:    pName,
				Total:   pTotal,
				Exec:    pc.Exec,
				Skipped: pc.Skip,
				Pct:     pct,
				Actions: actions,
			})
		}

		resp.Summary = watchSummary{
			Profiles:     profiles,
			GrandTotal:   grandTotal,
			GrandExec:    grandExec,
			GrandSkipped: grandSkip,
		}
	}

	// Build breakdown.
	resp.Hourly = computeBreakdown(entries, start, end, rangeType, loc)

	// Build approximate time spent.
	resp.TimeSpent = computeTimeSpentRange(entries, start, end, loc)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}

// handleStats returns event log metadata: storage backend, file size, entry count, and date range.
func handleStats(w http.ResponseWriter, r *http.Request) {
	var stats logStats

	switch eventlog.Default.(type) {
	case *eventlog.SQLiteStore:
		stats.Storage = "sqlite"
	default:
		stats.Storage = "file"
	}

	if info, err := os.Stat(eventlog.Default.Path()); err == nil {
		stats.FileSize = info.Size()
	}

	entries, _ := eventlog.Entries(0)
	stats.Entries = len(entries)
	if len(entries) > 0 {
		stats.OldestEntry = entries[0].Time.Format(time.RFC3339)
		stats.NewestEntry = entries[len(entries)-1].Time.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(stats)
}

// handleVoice returns frequently-used TTS texts with their usage counts and cache status.
func handleVoice(w http.ResponseWriter, r *http.Request) {
	days := 0
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}

	lines, _ := eventlog.VoiceLines(days)
	if len(lines) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(voiceResponse{Lines: []voiceLine{}, Total: 0})
		return
	}

	// Load voice cache to check which texts have pre-generated WAVs.
	cache, _ := voice.OpenCache()

	total := 0
	for _, l := range lines {
		total += l.Count
	}

	out := make([]voiceLine, len(lines))
	for i, l := range lines {
		pct := 0
		if total > 0 {
			pct = l.Count * 100 / total
		}
		vl := voiceLine{
			Rank:  i + 1,
			Text:  l.Text,
			Count: l.Count,
			Pct:   pct,
		}
		if cache != nil {
			if _, ok := cache.Lookup(l.Text); ok {
				hash := voice.TextHash(l.Text)
				vl.Cached = true
				vl.Hash = hash
				if e, ok := cache.Entries[hash]; ok {
					vl.Voice = e.Voice
				}
			}
		}
		out[i] = vl
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(voiceResponse{Lines: out, Total: total})
}

// handleVoicePlay serves a cached AI voice WAV file by its content hash.
func handleVoicePlay(w http.ResponseWriter, r *http.Request) {
	// URL: /api/voice/play/{hash}
	hash := r.URL.Path[len("/api/voice/play/"):]
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}

	cache, err := voice.OpenCache()
	if err != nil {
		http.Error(w, "voice cache unavailable", http.StatusInternalServerError)
		return
	}

	entry, ok := cache.Entries[hash]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	wavPath := filepath.Join(cache.Dir, entry.Hash+".wav")
	data, err := os.ReadFile(wavPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

type silentResponse struct {
	Active bool    `json:"active"`
	Until  *string `json:"until"`
}

// handleSilent reads (GET) or updates (POST) the silent/suppress mode.
// POST accepts {"minutes": N} to enable or {"disable": true} to turn off.
func handleSilent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var resp silentResponse
		if t, ok := silent.SilentUntil(); ok {
			s := t.Format(time.RFC3339)
			resp.Active = true
			resp.Until = &s
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)

	case http.MethodPost:
		var req struct {
			Minutes int  `json:"minutes"`
			Disable bool `json:"disable"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Disable {
			silent.Disable()
			eventlog.LogSilentDisable()
		} else if req.Minutes > 0 {
			d := time.Duration(req.Minutes) * time.Minute
			silent.Enable(d)
			eventlog.LogSilentEnable(d)
		} else {
			http.Error(w, "provide minutes > 0 or disable: true", http.StatusBadRequest)
			return
		}

		// Return updated status.
		var resp silentResponse
		if t, ok := silent.SilentUntil(); ok {
			s := t.Format(time.RFC3339)
			resp.Active = true
			resp.Until = &s
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}



type triggerRequest struct {
	Profile string `json:"profile"`
	Action  string `json:"action"`
	Volume  *int   `json:"volume"`
	Log     *bool  `json:"log"`
}

type triggerResponse struct {
	OK        bool   `json:"ok"`
	Profile   string `json:"profile,omitempty"`
	Action    string `json:"action,omitempty"`
	StepsRun  int    `json:"steps_run,omitempty"`
	StepsTotal int   `json:"steps_total,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleTrigger executes a real notification pipeline from the web UI. It reloads
// the config, resolves the profile/action, applies cooldown and silent checks, runs
// the step pipeline, and logs the result -- mirroring the CLI's runAction path.
func handleTrigger(configPath string, fallback config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadCfg(configPath, fallback)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req triggerRequest
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(triggerResponse{Error: "invalid JSON"})
				return
			}
		} else {
			// GET: read from query params.
			req.Profile = r.URL.Query().Get("profile")
			req.Action = r.URL.Query().Get("action")
			if v := r.URL.Query().Get("volume"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100 {
					req.Volume = &n
				}
			}
			if v := r.URL.Query().Get("log"); v == "false" {
				b := false
				req.Log = &b
			}
		}

		if req.Profile == "" {
			req.Profile = "default"
		}
		if req.Action == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(triggerResponse{Error: "action is required"})
			return
		}

		// Silent mode check.
		if silent.IsSilent() {
			if req.Log == nil || *req.Log {
				eventlog.LogSilent(req.Profile, req.Action)
			}
			json.NewEncoder(w).Encode(triggerResponse{
				OK:      true,
				Profile: req.Profile,
				Action:  req.Action,
			})
			return
		}

		// Resolve profile + action.
		resolved, act, err := config.Resolve(cfg, req.Profile, req.Action)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(triggerResponse{Error: err.Error()})
			return
		}

		// Cooldown check.
		cdSec := act.CooldownSeconds
		if cdSec == 0 {
			cdSec = cfg.Options.CooldownSeconds
		}
		cdEnabled := cfg.Options.Cooldown
		if cdEnabled && cdSec > 0 && cooldown.Check(resolved, req.Action, cdSec) {
			if req.Log == nil || *req.Log {
				eventlog.LogCooldown(resolved, req.Action, cdSec)
			}
			json.NewEncoder(w).Encode(triggerResponse{
				OK:      true,
				Profile: resolved,
				Action:  req.Action,
			})
			return
		}

		// AFK detection.
		afk := false
		if cfg.Options.AFKThresholdSeconds > 0 {
			if idleSec, err := idle.IdleSeconds(); err == nil {
				afk = idleSec >= float64(cfg.Options.AFKThresholdSeconds)
			}
		}

		// Merge credentials.
		creds := config.MergeCredentials(cfg.Options.Credentials, cfg.Profiles[resolved].Credentials)

		// Volume: request override → config default.
		vol := cfg.Options.DefaultVolume
		if req.Volume != nil {
			vol = *req.Volume
		}

		// Build template vars.
		host, _ := os.Hostname()
		now := time.Now()
		vars := tmpl.Vars{
			Profile:  resolved,
			Time:     now.Format("15:04"),
			TimeSay:  now.Format("3:04 PM"),
			Date:     now.Format("2006-01-02"),
			DateSay:  now.Format("January 2, 2006"),
			Hostname: host,
		}

		// Filter and execute steps.
		desk := cfg.Profiles[resolved].Desktop
		totalSteps := len(act.Steps)
		filtered := runner.FilterSteps(act.Steps, afk, false, 0)
		execErr := runner.Execute(filtered, vol, creds, vars, desk)

		// Record cooldown.
		if cdEnabled && cdSec > 0 {
			cooldown.Record(resolved, req.Action)
			if req.Log == nil || *req.Log {
				eventlog.LogCooldownRecord(resolved, req.Action, cdSec)
			}
		}

		// Log execution.
		if req.Log == nil || *req.Log {
			eventlog.Log(req.Action, filtered, afk, vars, desk)
		}

		if execErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(triggerResponse{Error: execErr.Error()})
			return
		}

		json.NewEncoder(w).Encode(triggerResponse{
			OK:         true,
			Profile:    resolved,
			Action:     req.Action,
			StepsRun:   len(filtered),
			StepsTotal: totalSteps,
		})
	}
}

// redactConfig returns a JSON-safe representation of the config with
// credential values replaced by "***".
func redactConfig(cfg config.Config) config.Config {
	out := cfg
	out.Options.Credentials = redactCreds(cfg.Options.Credentials)

	out.Profiles = make(map[string]config.Profile, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		cp := p
		if p.Credentials != nil {
			redacted := redactCreds(*p.Credentials)
			cp.Credentials = &redacted
		}
		out.Profiles[name] = cp
	}
	return out
}

func redactCreds(c config.Credentials) config.Credentials {
	if c.DiscordWebhook != "" {
		c.DiscordWebhook = "***"
	}
	if c.SlackWebhook != "" {
		c.SlackWebhook = "***"
	}
	if c.TelegramToken != "" {
		c.TelegramToken = "***"
	}
	if c.TelegramChatID != "" {
		c.TelegramChatID = "***"
	}
	if c.OpenAIAPIKey != "" {
		c.OpenAIAPIKey = "***"
	}
	return c
}

// credentialRequirements maps step types to the credential fields they need.
var credentialRequirements = map[string][]string{
	"discord":        {"discord_webhook"},
	"discord_voice":  {"discord_webhook"},
	"slack":          {"slack_webhook"},
	"telegram":       {"telegram_token", "telegram_chat_id"},
	"telegram_audio": {"telegram_token", "telegram_chat_id"},
	"telegram_voice": {"telegram_token", "telegram_chat_id"},
}

// handleCredentials reports which credentials each profile needs and whether
// they are configured ("ok") or "missing", without revealing actual values.
func handleCredentials(configPath string, fallback config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadCfg(configPath, fallback)
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)

		var result []profileCreds
		for _, name := range names {
			p := cfg.Profiles[name]
			merged := config.MergeCredentials(cfg.Options.Credentials, p.Credentials)

			// Collect unique credential types needed by steps in this profile.
			needed := map[string]bool{}
			for _, action := range p.Actions {
				for _, step := range action.Steps {
					if reqs, ok := credentialRequirements[step.Type]; ok {
						for _, req := range reqs {
							needed[req] = true
						}
					}
				}
			}

			if len(needed) == 0 {
				continue
			}

			// Check each needed credential.
			credTypes := make([]string, 0, len(needed))
			for ct := range needed {
				credTypes = append(credTypes, ct)
			}
			sort.Strings(credTypes)

			var creds []credStatus
			for _, ct := range credTypes {
				status := "missing"
				switch ct {
				case "discord_webhook":
					if merged.DiscordWebhook != "" {
						status = "ok"
					}
				case "slack_webhook":
					if merged.SlackWebhook != "" {
						status = "ok"
					}
				case "telegram_token":
					if merged.TelegramToken != "" {
						status = "ok"
					}
				case "telegram_chat_id":
					if merged.TelegramChatID != "" {
						status = "ok"
					}
				}
				creds = append(creds, credStatus{Type: ct, Status: status})
			}
			result = append(result, profileCreds{Profile: name, Credentials: creds})
		}

		if result == nil {
			result = []profileCreds{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(result)
	}
}


