package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/tmpl"
)

//go:embed static/index.html
var staticFS embed.FS

// Serve starts the dashboard HTTP server on 127.0.0.1:port and blocks
// until interrupted. The config is loaded once at startup.
func Serve(cfg config.Config, configPath string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/config", handleConfig(cfg))
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/summary", handleSummary)
	mux.HandleFunc("/api/events", handleEvents)
	mux.HandleFunc("/api/test", handleTest(cfg))
	mux.HandleFunc("/api/watch", handleWatch)

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

	fmt.Printf("Dashboard: http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
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
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}

	entries := loadEntries(days)

	type jsonEntry struct {
		Time    string `json:"time"`
		Profile string `json:"profile"`
		Action  string `json:"action"`
		Kind    string `json:"kind"`
	}

	out := make([]jsonEntry, len(entries))
	for i, e := range entries {
		out[i] = jsonEntry{
			Time:    e.Time.Format(time.RFC3339),
			Profile: e.Profile,
			Action:  e.Action,
			Kind:    eventlog.KindString(e.Kind),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func handleSummary(w http.ResponseWriter, r *http.Request) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}

	entries := loadEntries(0) // load all, SummarizeByDay handles filtering
	groups := eventlog.SummarizeByDay(entries, days)

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

			type jsonEntry struct {
				Time    string `json:"time"`
				Profile string `json:"profile"`
				Action  string `json:"action"`
				Kind    string `json:"kind"`
			}
			out := make([]jsonEntry, len(entries))
			for i, e := range entries {
				out[i] = jsonEntry{
					Time:    e.Time.Format(time.RFC3339),
					Profile: e.Profile,
					Action:  e.Action,
					Kind:    eventlog.KindString(e.Kind),
				}
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
	type watchResponse struct {
		Date    string       `json:"date"`
		DayName string       `json:"day_name"`
		IsToday bool         `json:"is_today"`
		Summary watchSummary `json:"summary"`
		Hourly  watchHourly  `json:"hourly"`
	}

	resp := watchResponse{
		Date:    targetDate.Format("2006-01-02"),
		DayName: targetDate.Format("Monday"),
		IsToday: isToday,
		Summary: watchSummary{Profiles: []watchProfile{}},
		Hourly:  watchHourly{Profiles: []string{}, Hours: []watchHourRow{}, ProfileTotals: []int{}},
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

	// Build summary (mirrors aggregateGroups from cmd/notify/history.go).
	if len(groups) > 0 {
		type actionKey struct{ profile, action string }
		type counts struct{ exec, skip int }

		perAction := map[actionKey]*counts{}
		perProfile := map[string]*counts{}
		var profileOrder []string
		profileSeen := map[string]bool{}

		for _, dg := range groups {
			for _, s := range dg.Summaries {
				ak := actionKey{s.Profile, s.Action}
				ac, ok := perAction[ak]
				if !ok {
					ac = &counts{}
					perAction[ak] = ac
				}
				ac.exec += s.Executions
				ac.skip += s.Skipped

				pc, ok := perProfile[s.Profile]
				if !ok {
					pc = &counts{}
					perProfile[s.Profile] = pc
				}
				pc.exec += s.Executions
				pc.skip += s.Skipped

				if !profileSeen[s.Profile] {
					profileSeen[s.Profile] = true
					profileOrder = append(profileOrder, s.Profile)
				}
			}
		}
		sort.Strings(profileOrder)

		actionsByProfile := map[string][]actionKey{}
		for ak := range perAction {
			actionsByProfile[ak.profile] = append(actionsByProfile[ak.profile], ak)
		}
		for _, aks := range actionsByProfile {
			sort.Slice(aks, func(i, j int) bool { return aks[i].action < aks[j].action })
		}

		grandExec, grandSkip := 0, 0
		for _, pc := range perProfile {
			grandExec += pc.exec
			grandSkip += pc.skip
		}
		grandTotal := grandExec + grandSkip

		profiles := make([]watchProfile, 0, len(profileOrder))
		for _, pName := range profileOrder {
			pc := perProfile[pName]
			pTotal := pc.exec + pc.skip
			pct := 0
			if grandTotal > 0 {
				pct = pTotal * 100 / grandTotal
			}

			actions := make([]watchAction, 0, len(actionsByProfile[pName]))
			for _, ak := range actionsByProfile[pName] {
				ac := perAction[ak]
				actions = append(actions, watchAction{
					Name:    ak.action,
					Total:   ac.exec + ac.skip,
					Exec:    ac.exec,
					Skipped: ac.skip,
				})
			}

			profiles = append(profiles, watchProfile{
				Name:    pName,
				Total:   pTotal,
				Exec:    pc.exec,
				Skipped: pc.skip,
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

	// Build hourly (mirrors renderHourlyTable from cmd/notify/history.go).
	type hp struct {
		hour    int
		profile string
	}
	perCell := map[hp]int{}
	perHour := map[int]int{}
	profileSet := map[string]bool{}
	minHour, maxHour := 24, -1

	for _, e := range entries {
		local := e.Time.In(loc)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		if !day.Equal(targetDate) || e.Kind == eventlog.KindOther {
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

	if len(perCell) > 0 {
		hProfiles := make([]string, 0, len(profileSet))
		for p := range profileSet {
			hProfiles = append(hProfiles, p)
		}
		sort.Strings(hProfiles)

		hGrandTotal := 0
		for _, c := range perHour {
			hGrandTotal += c
		}

		hours := make([]watchHourRow, 0, maxHour-minHour+1)
		profileTotals := make([]int, len(hProfiles))

		for h := minHour; h <= maxHour; h++ {
			cnts := make([]int, len(hProfiles))
			for i, p := range hProfiles {
				c := perCell[hp{h, p}]
				cnts[i] = c
				profileTotals[i] += c
			}
			ht := perHour[h]
			pct := 0
			if hGrandTotal > 0 {
				pct = ht * 100 / hGrandTotal
			}
			hours = append(hours, watchHourRow{
				Hour:   h,
				Counts: cnts,
				Total:  ht,
				Pct:    pct,
			})
		}

		resp.Hourly = watchHourly{
			Profiles:      hProfiles,
			Hours:         hours,
			ProfileTotals: profileTotals,
			GrandTotal:    hGrandTotal,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
	return c
}


