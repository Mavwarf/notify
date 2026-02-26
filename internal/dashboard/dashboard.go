package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/silent"
	"github.com/Mavwarf/notify/internal/tmpl"
)

//go:embed static/index.html
var staticFS embed.FS

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

type watchHourRow struct {
	Hour   int   `json:"hour"`
	Counts []int `json:"counts"`
	Total  int   `json:"total"`
	Pct    int   `json:"pct"`
}

type watchHourly struct {
	Profiles      []string       `json:"profiles"`
	Hours         []watchHourRow `json:"hours"`
	ProfileTotals []int          `json:"profile_totals"`
	GrandTotal    int            `json:"grand_total"`
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
	Date      string         `json:"date"`
	DayName   string         `json:"day_name"`
	IsToday   bool           `json:"is_today"`
	Summary   watchSummary   `json:"summary"`
	Hourly    watchHourly    `json:"hourly"`
	TimeSpent watchTimeSpent `json:"time_spent"`
}

type logStats struct {
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
	Rank  int    `json:"rank"`
	Text  string `json:"text"`
	Count int    `json:"count"`
	Pct   int    `json:"pct"`
}

type voiceResponse struct {
	Lines []voiceLine `json:"lines"`
	Total int         `json:"total"`
}

// Serve starts the dashboard HTTP server on 127.0.0.1:port and blocks
// until interrupted. If open is true, a browser window is launched in
// app mode (chromeless) pointing at the dashboard URL.
func Serve(cfg config.Config, configPath string, port int, open bool) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/config", handleConfig(cfg))
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/summary", handleSummary)
	mux.HandleFunc("/api/events", handleEvents)
	mux.HandleFunc("/api/test", handleTest(cfg))
	mux.HandleFunc("/api/credentials", handleCredentials(cfg))
	mux.HandleFunc("/api/watch", handleWatch)
	mux.HandleFunc("/api/stats", handleStats)
	mux.HandleFunc("/api/voice", handleVoice)
	mux.HandleFunc("/api/silent", handleSilent)

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
	fmt.Printf("Dashboard: %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleConfig(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redacted := redactConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(redacted)
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	var entries []eventlog.Entry
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			entries = loadEntriesByHours(v)
		} else {
			entries = loadEntries(7)
		}
	} else {
		days := 7
		if d := r.URL.Query().Get("days"); d != "" {
			if v, err := strconv.Atoi(d); err == nil && v >= 0 {
				days = v
			}
		}
		entries = loadEntries(days)
	}

	out := make([]jsonEntry, len(entries))
	for i, e := range entries {
		out[i] = entryToJSON(e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func handleSummary(w http.ResponseWriter, r *http.Request) {
	var entries []eventlog.Entry
	if h := r.URL.Query().Get("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			entries = loadEntriesByHours(v)
		} else {
			entries = loadEntries(0)
		}
	} else {
		entries = loadEntries(0)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

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

	logPath := eventlog.LogPath()
	var offset int64

	// Start from end of file.
	if info, err := os.Stat(logPath); err == nil {
		offset = info.Size()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(logPath)
			if err != nil {
				// File removed (history clear), reset offset.
				offset = 0
				continue
			}

			// File truncated (history clear while running).
			if info.Size() < offset {
				offset = 0
			}

			if info.Size() <= offset {
				continue
			}

			f, err := os.Open(logPath)
			if err != nil {
				continue
			}

			f.Seek(offset, io.SeekStart)
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				continue
			}
			offset += int64(len(data))

			entries := eventlog.ParseEntries(string(data))
			if len(entries) == 0 {
				continue
			}

			out := make([]jsonEntry, len(entries))
			for i, e := range entries {
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

func handleTest(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

			wouldRun := runner.FilteredIndices(act.Steps, false, false)
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func handleWatch(w http.ResponseWriter, r *http.Request) {
	entries := loadEntries(0)

	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Allow browsing any day via ?date=YYYY-MM-DD.
	targetDate := today
	isToday := true
	if d := r.URL.Query().Get("date"); d != "" {
		if t, err := time.ParseInLocation("2006-01-02", d, loc); err == nil {
			targetDate = t
			isToday = targetDate.Equal(today)
		}
	}

	groups := eventlog.SummarizeByDay(entries, 0) // load all days, filter below

	resp := watchResponse{
		Date:      targetDate.Format("2006-01-02"),
		DayName:   targetDate.Format("Monday"),
		IsToday:   isToday,
		Summary:   watchSummary{Profiles: []watchProfile{}},
		Hourly:    watchHourly{Profiles: []string{}, Hours: []watchHourRow{}, ProfileTotals: []int{}},
		TimeSpent: watchTimeSpent{Profiles: []watchTimeProfile{}},
	}

	// Filter groups to the target date only.
	var filteredGroups []eventlog.DayGroup
	for _, g := range groups {
		gDay := time.Date(g.Date.Year(), g.Date.Month(), g.Date.Day(), 0, 0, 0, 0, loc)
		if gDay.Equal(targetDate) {
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

	// Build hourly using shared computation.
	hd := eventlog.ComputeHourly(entries, targetDate, loc)
	if len(hd.PerCell) > 0 {
		hours := make([]watchHourRow, 0, hd.MaxHour-hd.MinHour+1)
		for h := hd.MinHour; h <= hd.MaxHour; h++ {
			cnts := make([]int, len(hd.Profiles))
			for i, p := range hd.Profiles {
				cnts[i] = hd.PerCell[eventlog.HourProfile{Hour: h, Profile: p}]
			}
			ht := hd.PerHour[h]
			pct := 0
			if hd.GrandTotal > 0 {
				pct = ht * 100 / hd.GrandTotal
			}
			hours = append(hours, watchHourRow{
				Hour:   h,
				Counts: cnts,
				Total:  ht,
				Pct:    pct,
			})
		}

		resp.Hourly = watchHourly{
			Profiles:      hd.Profiles,
			Hours:         hours,
			ProfileTotals: hd.ProfileTotals,
			GrandTotal:    hd.GrandTotal,
		}
	}

	// Build approximate time spent using shared computation.
	tsd := eventlog.ComputeTimeSpent(entries, targetDate, loc)
	if len(tsd.Profiles) > 0 {
		tps := make([]watchTimeProfile, len(tsd.Profiles))
		for i, p := range tsd.Profiles {
			tps[i] = watchTimeProfile{Name: p.Name, Seconds: p.Seconds}
		}
		resp.TimeSpent = watchTimeSpent{Profiles: tps, Total: tsd.Total}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	var stats logStats

	logPath := eventlog.LogPath()
	if info, err := os.Stat(logPath); err == nil {
		stats.FileSize = info.Size()
	}

	entries := loadEntries(0)
	stats.Entries = len(entries)
	if len(entries) > 0 {
		stats.OldestEntry = entries[0].Time.Format(time.RFC3339)
		stats.NewestEntry = entries[len(entries)-1].Time.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleVoice(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(eventlog.LogPath())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(voiceResponse{Lines: []voiceLine{}, Total: 0})
		return
	}

	content := string(data)

	// Optional ?days=N filter (0 = all time, default).
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			content = eventlog.FilterBlocksByDays(content, v)
		}
	}

	lines := eventlog.ParseVoiceLines(content)

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
		out[i] = voiceLine{
			Rank:  i + 1,
			Text:  l.Text,
			Count: l.Count,
			Pct:   pct,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(voiceResponse{Lines: out, Total: total})
}

type silentResponse struct {
	Active bool    `json:"active"`
	Until  *string `json:"until"`
}

func handleSilent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var resp silentResponse
		if t, ok := silent.SilentUntil(); ok {
			s := t.Format(time.RFC3339)
			resp.Active = true
			resp.Until = &s
		}
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}


// loadEntriesByHours reads and parses the event log, filtering to entries
// from the last N hours.
func loadEntriesByHours(hours int) []eventlog.Entry {
	data, err := os.ReadFile(eventlog.LogPath())
	if err != nil {
		return nil
	}
	entries := eventlog.ParseEntries(string(data))
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var filtered []eventlog.Entry
	for _, e := range entries {
		if !e.Time.Before(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// loadEntries reads and parses the event log, filtering to the last N days.
// Pass days=0 to load all entries.
func loadEntries(days int) []eventlog.Entry {
	data, err := os.ReadFile(eventlog.LogPath())
	if err != nil {
		return nil
	}
	entries := eventlog.ParseEntries(string(data))
	if days <= 0 {
		return entries
	}

	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(days - 1))

	var filtered []eventlog.Entry
	for _, e := range entries {
		if !e.Time.Before(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered
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

func handleCredentials(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}


