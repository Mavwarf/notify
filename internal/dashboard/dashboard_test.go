package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/eventlog"
)

func testConfig() config.Config {
	vol := 80
	return config.Config{
		Options: config.Options{
			AFKThresholdSeconds: 300,
			DefaultVolume:       100,
			Credentials: config.Credentials{
				DiscordWebhook: "https://discord.com/api/webhooks/secret",
				SlackWebhook:   "https://hooks.slack.com/secret",
				TelegramToken:  "bot-token-secret",
				TelegramChatID: "12345",
			},
		},
		Profiles: map[string]config.Profile{
			"notify": {
				Actions: map[string]config.Action{
					"ready": {
						Steps: []config.Step{
							{Type: "sound", Sound: "success"},
							{Type: "say", Text: "Ready!"},
							{Type: "discord", Text: "Ready!", When: "afk"},
						},
					},
					"error": {
						Steps: []config.Step{
							{Type: "sound", Sound: "error"},
						},
					},
				},
			},
			"boss": {
				Aliases: []string{"b"},
				Credentials: &config.Credentials{
					DiscordWebhook: "https://discord.com/api/webhooks/boss-secret",
				},
				Actions: map[string]config.Action{
					"ready": {
						Steps: []config.Step{
							{Type: "sound", Sound: "notification", Volume: &vol},
							{Type: "say", Text: "Boss is ready"},
						},
					},
				},
			},
			"romans": {
				Actions: map[string]config.Action{
					"ready": {
						Steps: []config.Step{
							{Type: "sound", Sound: "success"},
							{Type: "telegram", Text: "Romans ready!"},
						},
					},
				},
			},
		},
	}
}

func TestHandleIndex(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "notify dashboard") {
		t.Fatal("expected HTML to contain 'notify dashboard'")
	}
}

func TestHandleIndex404(t *testing.T) {
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleConfigRedacted(t *testing.T) {
	cfg := testConfig()
	handler := handleConfig(cfg)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Must NOT contain actual secrets.
	if strings.Contains(body, "secret") {
		t.Fatal("response contains unredacted credentials")
	}
	if strings.Contains(body, "bot-token-secret") {
		t.Fatal("response contains unredacted telegram token")
	}

	// Must contain redacted markers.
	if !strings.Contains(body, `"***"`) {
		t.Fatal("expected redacted '***' in response")
	}

	// Verify it's valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestHandleHistory(t *testing.T) {
	// Create a temporary log file.
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	now := time.Now()
	ts := now.Format(time.RFC3339)

	content := fmt.Sprintf(`%s  profile=notify  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=success
%s    step[2] say  text="Ready!"

%s  profile=boss  action=ready  steps=sound  afk=true
%s    step[1] sound  sound=notification

`, ts, ts, ts, ts, ts)

	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Override LogPath for this test.
	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	req := httptest.NewRequest("GET", "/api/history?days=1", nil)
	w := httptest.NewRecorder()
	handleHistory(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entries []struct {
		Time    string `json:"time"`
		Profile string `json:"profile"`
		Action  string `json:"action"`
		Kind    string `json:"kind"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestHandleTestEndpoint(t *testing.T) {
	cfg := testConfig()
	handler := handleTest(cfg)

	body := `{"profile":"notify","action":"ready"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var results []struct {
		Action    string `json:"action"`
		TotalRun  int    `json:"total_run"`
		TotalSkip int    `json:"total_skip"`
		Steps     []struct {
			Index    int    `json:"index"`
			Type     string `json:"type"`
			WouldRun bool   `json:"would_run"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 action result, got %d", len(results))
	}

	r := results[0]
	if r.Action != "ready" {
		t.Fatalf("expected action 'ready', got %q", r.Action)
	}
	if len(r.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(r.Steps))
	}
}

func TestHandleTestAllActions(t *testing.T) {
	cfg := testConfig()
	handler := handleTest(cfg)

	// Empty action = show all actions for profile.
	body := `{"profile":"notify","action":""}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var results []struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 action results (ready, error), got %d", len(results))
	}
}

func TestHandleTestFallback(t *testing.T) {
	cfg := testConfig()
	handler := handleTest(cfg)

	// "nonexistent" profile with action "ready" — should fall back to default
	// profile. Since testConfig has no "default" profile, Resolve returns
	// nothing and the result is an empty list.
	body := `{"profile":"nonexistent","action":"ready"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Add a "default" profile and verify fallback works.
	cfg.Profiles["default"] = config.Profile{
		Actions: map[string]config.Action{
			"ready": {
				Steps: []config.Step{{Type: "sound", Sound: "default-ready"}},
			},
		},
	}
	handler2 := handleTest(cfg)

	body2 := `{"profile":"unknown","action":"ready"}`
	req2 := httptest.NewRequest("POST", "/api/test", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler2(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var results []struct {
		Action   string `json:"action"`
		Resolved string `json:"resolved"`
		Steps    []struct {
			Type string `json:"type"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result via fallback, got %d", len(results))
	}
	if results[0].Resolved != "default" {
		t.Fatalf("expected resolved='default', got %q", results[0].Resolved)
	}
}

func TestHandleTestMethodNotAllowed(t *testing.T) {
	cfg := testConfig()
	handler := handleTest(cfg)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRedactConfig(t *testing.T) {
	cfg := testConfig()
	redacted := redactConfig(cfg)

	// Global credentials redacted.
	if redacted.Options.Credentials.DiscordWebhook != "***" {
		t.Fatal("global discord webhook not redacted")
	}
	if redacted.Options.Credentials.SlackWebhook != "***" {
		t.Fatal("global slack webhook not redacted")
	}
	if redacted.Options.Credentials.TelegramToken != "***" {
		t.Fatal("global telegram token not redacted")
	}
	if redacted.Options.Credentials.TelegramChatID != "***" {
		t.Fatal("global telegram chat ID not redacted")
	}

	// Profile credentials redacted.
	boss := redacted.Profiles["boss"]
	if boss.Credentials == nil {
		t.Fatal("boss profile credentials should not be nil")
	}
	if boss.Credentials.DiscordWebhook != "***" {
		t.Fatal("boss discord webhook not redacted")
	}

	// Original config must be unmodified.
	if cfg.Options.Credentials.DiscordWebhook == "***" {
		t.Fatal("original config was mutated")
	}
}

func TestHandleWatch(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	now := time.Now()
	ts := now.Format(time.RFC3339)

	content := fmt.Sprintf(`%s  profile=notify  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=success
%s    step[2] say  text="Ready!"

%s  profile=notify  action=error  steps=sound  afk=false
%s    step[1] sound  sound=error

%s  profile=boss  action=ready  steps=sound  afk=true
%s    step[1] sound  sound=notification

%s  profile=romans  action=ready  steps=sound,telegram  afk=false
%s    step[1] sound  sound=success
%s    step[2] telegram  text="Romans ready!"

%s  profile=notify  action=ready  cooldown=skipped (30s)

`, ts, ts, ts, ts, ts, ts, ts, ts, ts, ts, ts)

	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	req := httptest.NewRequest("GET", "/api/watch", nil)
	w := httptest.NewRecorder()
	handleWatch(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Date    string `json:"date"`
		DayName string `json:"day_name"`
		Summary struct {
			Profiles []struct {
				Name    string `json:"name"`
				Total   int    `json:"total"`
				Exec    int    `json:"exec"`
				Skipped int    `json:"skipped"`
				Pct     int    `json:"pct"`
				Actions []struct {
					Name    string `json:"name"`
					Total   int    `json:"total"`
					Exec    int    `json:"exec"`
					Skipped int    `json:"skipped"`
				} `json:"actions"`
			} `json:"profiles"`
			GrandTotal   int `json:"grand_total"`
			GrandExec    int `json:"grand_exec"`
			GrandSkipped int `json:"grand_skipped"`
		} `json:"summary"`
		Hourly struct {
			Profiles      []string `json:"profiles"`
			Hours         []struct {
				Hour   int   `json:"hour"`
				Counts []int `json:"counts"`
				Total  int   `json:"total"`
				Pct    int   `json:"pct"`
			} `json:"hours"`
			ProfileTotals []int `json:"profile_totals"`
			GrandTotal    int   `json:"grand_total"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify date fields.
	expected := now.Format("2006-01-02")
	if resp.Date != expected {
		t.Fatalf("expected date %q, got %q", expected, resp.Date)
	}
	if resp.DayName == "" {
		t.Fatal("expected non-empty day_name")
	}

	// Verify summary profiles.
	if len(resp.Summary.Profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(resp.Summary.Profiles))
	}

	// Grand totals: 4 executions + 1 cooldown skip = 5.
	if resp.Summary.GrandTotal != 5 {
		t.Fatalf("expected grand_total 5, got %d", resp.Summary.GrandTotal)
	}
	if resp.Summary.GrandExec != 4 {
		t.Fatalf("expected grand_exec 4, got %d", resp.Summary.GrandExec)
	}
	if resp.Summary.GrandSkipped != 1 {
		t.Fatalf("expected grand_skipped 1, got %d", resp.Summary.GrandSkipped)
	}

	// Verify boss profile (sorted first alphabetically).
	boss := resp.Summary.Profiles[0]
	if boss.Name != "boss" {
		t.Fatalf("expected first profile 'boss', got %q", boss.Name)
	}
	if boss.Total != 1 {
		t.Fatalf("expected boss total 1, got %d", boss.Total)
	}

	// Verify notify profile.
	notify := resp.Summary.Profiles[1]
	if notify.Name != "notify" {
		t.Fatalf("expected second profile 'notify', got %q", notify.Name)
	}
	if notify.Total != 3 {
		t.Fatalf("expected notify total 3, got %d", notify.Total)
	}
	if len(notify.Actions) != 2 {
		t.Fatalf("expected 2 actions for notify, got %d", len(notify.Actions))
	}

	// Verify romans profile.
	romans := resp.Summary.Profiles[2]
	if romans.Name != "romans" {
		t.Fatalf("expected third profile 'romans', got %q", romans.Name)
	}
	if romans.Total != 1 {
		t.Fatalf("expected romans total 1, got %d", romans.Total)
	}

	// Verify hourly section.
	if len(resp.Hourly.Profiles) != 3 {
		t.Fatalf("expected 3 hourly profiles, got %d", len(resp.Hourly.Profiles))
	}
	if len(resp.Hourly.Hours) == 0 {
		t.Fatal("expected at least 1 hourly row")
	}
	if resp.Hourly.GrandTotal != 5 {
		t.Fatalf("expected hourly grand_total 5, got %d", resp.Hourly.GrandTotal)
	}
}

func TestHandleWatchDateParam(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tsYesterday := yesterday.Format(time.RFC3339)
	tsToday := now.Format(time.RFC3339)

	content := fmt.Sprintf(`%s  profile=notify  action=ready  steps=sound  afk=false
%s    step[1] sound  sound=success

%s  profile=notify  action=error  steps=sound  afk=false
%s    step[1] sound  sound=error

`, tsYesterday, tsYesterday, tsToday, tsToday)

	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	// Request yesterday's data.
	dateStr := yesterday.Format("2006-01-02")
	req := httptest.NewRequest("GET", "/api/watch?date="+dateStr, nil)
	w := httptest.NewRecorder()
	handleWatch(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Date    string `json:"date"`
		IsToday bool   `json:"is_today"`
		Summary struct {
			Profiles     []struct{ Name string `json:"name"` } `json:"profiles"`
			GrandTotal   int                                    `json:"grand_total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Date != dateStr {
		t.Fatalf("expected date %q, got %q", dateStr, resp.Date)
	}
	if resp.IsToday {
		t.Fatal("expected is_today=false for yesterday")
	}
	// Only yesterday's entry should appear.
	if resp.Summary.GrandTotal != 1 {
		t.Fatalf("expected grand_total 1 for yesterday, got %d", resp.Summary.GrandTotal)
	}

	// Request today's data (no date param = today).
	req2 := httptest.NewRequest("GET", "/api/watch", nil)
	w2 := httptest.NewRecorder()
	handleWatch(w2, req2)

	var resp2 struct {
		IsToday bool `json:"is_today"`
		Summary struct {
			GrandTotal int `json:"grand_total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp2.IsToday {
		t.Fatal("expected is_today=true for default request")
	}
	if resp2.Summary.GrandTotal != 1 {
		t.Fatalf("expected grand_total 1 for today, got %d", resp2.Summary.GrandTotal)
	}
}

func TestHandleWatchEmpty(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	req := httptest.NewRequest("GET", "/api/watch", nil)
	w := httptest.NewRecorder()
	handleWatch(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Summary struct {
			Profiles []interface{} `json:"profiles"`
		} `json:"summary"`
		Hourly struct {
			Profiles []interface{} `json:"profiles"`
			Hours    []interface{} `json:"hours"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Should return empty arrays, not null.
	if resp.Summary.Profiles == nil {
		t.Fatal("summary.profiles should be [] not null")
	}
	if resp.Hourly.Profiles == nil {
		t.Fatal("hourly.profiles should be [] not null")
	}
	if resp.Hourly.Hours == nil {
		t.Fatal("hourly.hours should be [] not null")
	}
}

func TestHandleCredentials(t *testing.T) {
	cfg := testConfig()
	handler := handleCredentials(cfg)

	req := httptest.NewRequest("GET", "/api/credentials", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []struct {
		Profile     string `json:"profile"`
		Credentials []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// testConfig profiles:
	// - "boss" has discord steps → discord_webhook from profile creds → ok
	// - "notify" has discord step → discord_webhook from global creds → ok
	// - "romans" has telegram step → telegram_token + telegram_chat_id from global → ok
	// "boss" has only sound+say steps (ready action), so discord comes from "notify"
	// Wait — let's check: boss has sound+say only, notify has sound+say+discord, romans has sound+telegram.
	// So: boss has no remote steps → not in result.
	//     notify needs discord_webhook → ok (global creds).
	//     romans needs telegram_token + telegram_chat_id → ok (global creds).

	// boss profile only has sound+say steps, no remote → should NOT appear.
	for _, pc := range result {
		if pc.Profile == "boss" {
			t.Fatal("boss should not appear — only has sound/say steps")
		}
	}

	// Find notify profile.
	var notifyResult *struct {
		Profile     string `json:"profile"`
		Credentials []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"credentials"`
	}
	for i := range result {
		if result[i].Profile == "notify" {
			notifyResult = &result[i]
			break
		}
	}
	if notifyResult == nil {
		t.Fatal("expected notify profile in result")
	}
	if len(notifyResult.Credentials) != 1 {
		t.Fatalf("expected 1 credential for notify, got %d", len(notifyResult.Credentials))
	}
	if notifyResult.Credentials[0].Type != "discord_webhook" {
		t.Fatalf("expected discord_webhook, got %q", notifyResult.Credentials[0].Type)
	}
	if notifyResult.Credentials[0].Status != "ok" {
		t.Fatalf("expected ok status for discord_webhook, got %q", notifyResult.Credentials[0].Status)
	}

	// Find romans profile (telegram).
	var romansResult *struct {
		Profile     string `json:"profile"`
		Credentials []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"credentials"`
	}
	for i := range result {
		if result[i].Profile == "romans" {
			romansResult = &result[i]
			break
		}
	}
	if romansResult == nil {
		t.Fatal("expected romans profile in result")
	}
	if len(romansResult.Credentials) != 2 {
		t.Fatalf("expected 2 credentials for romans, got %d", len(romansResult.Credentials))
	}
	// Sorted: telegram_chat_id, telegram_token.
	if romansResult.Credentials[0].Type != "telegram_chat_id" || romansResult.Credentials[0].Status != "ok" {
		t.Fatalf("expected telegram_chat_id ok, got %q %q", romansResult.Credentials[0].Type, romansResult.Credentials[0].Status)
	}
	if romansResult.Credentials[1].Type != "telegram_token" || romansResult.Credentials[1].Status != "ok" {
		t.Fatalf("expected telegram_token ok, got %q %q", romansResult.Credentials[1].Type, romansResult.Credentials[1].Status)
	}
}

func TestHandleCredentialsMissing(t *testing.T) {
	// Config with no global credentials and a profile that needs discord.
	cfg := config.Config{
		Profiles: map[string]config.Profile{
			"test": {
				Actions: map[string]config.Action{
					"alert": {
						Steps: []config.Step{
							{Type: "discord", Text: "Hello"},
							{Type: "slack", Text: "Hello"},
						},
					},
				},
			},
		},
	}
	handler := handleCredentials(cfg)

	req := httptest.NewRequest("GET", "/api/credentials", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []struct {
		Profile     string `json:"profile"`
		Credentials []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(result))
	}
	if len(result[0].Credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(result[0].Credentials))
	}
	for _, c := range result[0].Credentials {
		if c.Status != "missing" {
			t.Fatalf("expected missing status for %q, got %q", c.Type, c.Status)
		}
	}
}

func TestHandleVoice(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	now := time.Now()
	ts := now.Format(time.RFC3339)
	yesterday := now.AddDate(0, 0, -1).Format(time.RFC3339)
	old := now.AddDate(0, 0, -30).Format(time.RFC3339)

	content := fmt.Sprintf(`%s  profile=notify  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=success
%s    step[2] say  text="Build complete"

%s  profile=notify  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=success
%s    step[2] say  text="Build complete"

%s  profile=boss  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=notification
%s    step[2] say  text="Boss is ready"

%s  profile=notify  action=error  steps=sound,say  afk=false
%s    step[1] sound  sound=error
%s    step[2] say  text="Build failed"

`, ts, ts, ts, yesterday, yesterday, yesterday, yesterday, yesterday, yesterday, old, old, old)

	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	// Test all time.
	req := httptest.NewRequest("GET", "/api/voice", nil)
	w := httptest.NewRecorder()
	handleVoice(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp voiceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Total != 4 {
		t.Fatalf("expected total 4, got %d", resp.Total)
	}
	if len(resp.Lines) != 3 {
		t.Fatalf("expected 3 unique lines, got %d", len(resp.Lines))
	}
	// "Build complete" has count 2, should be first (highest).
	if resp.Lines[0].Text != "Build complete" {
		t.Fatalf("expected first line 'Build complete', got %q", resp.Lines[0].Text)
	}
	if resp.Lines[0].Count != 2 {
		t.Fatalf("expected count 2 for 'Build complete', got %d", resp.Lines[0].Count)
	}
	if resp.Lines[0].Rank != 1 {
		t.Fatalf("expected rank 1, got %d", resp.Lines[0].Rank)
	}

	// Test with days filter — last 7 days should exclude the 30-day-old entry.
	req2 := httptest.NewRequest("GET", "/api/voice?days=7", nil)
	w2 := httptest.NewRecorder()
	handleVoice(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var resp2 voiceResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp2.Total != 3 {
		t.Fatalf("expected total 3 for 7-day filter, got %d", resp2.Total)
	}
	if len(resp2.Lines) != 2 {
		t.Fatalf("expected 2 unique lines for 7-day filter, got %d", len(resp2.Lines))
	}
}

func TestHandleVoiceEmpty(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notify.log")

	origPath := eventlog.LogPath
	eventlog.LogPath = func() string { return logFile }
	defer func() { eventlog.LogPath = origPath }()

	req := httptest.NewRequest("GET", "/api/voice", nil)
	w := httptest.NewRecorder()
	handleVoice(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp voiceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Lines == nil {
		t.Fatal("lines should be [] not null")
	}
	if len(resp.Lines) != 0 {
		t.Fatalf("expected 0 lines, got %d", len(resp.Lines))
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
}

func TestProfileMarshalJSON(t *testing.T) {
	cfg := testConfig()
	data, err := json.Marshal(cfg.Profiles["boss"])
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	// Should have aliases, credentials, and ready action.
	if _, ok := m["aliases"]; !ok {
		t.Fatal("missing 'aliases' key in marshaled profile")
	}
	if _, ok := m["credentials"]; !ok {
		t.Fatal("missing 'credentials' key in marshaled profile")
	}
	if _, ok := m["ready"]; !ok {
		t.Fatal("missing 'ready' action key in marshaled profile")
	}
	// Should NOT have extends (it's empty).
	if _, ok := m["extends"]; ok {
		t.Fatal("should not have 'extends' key when empty")
	}
}
