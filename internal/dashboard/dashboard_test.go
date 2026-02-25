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
			"default": {
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

	content := fmt.Sprintf(`%s  profile=default  action=ready  steps=sound,say  afk=false
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

	body := `{"profile":"default","action":"ready"}`
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
	body := `{"profile":"default","action":""}`
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

func TestHandleTestNotFound(t *testing.T) {
	cfg := testConfig()
	handler := handleTest(cfg)

	body := `{"profile":"nonexistent","action":"ready"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
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

	content := fmt.Sprintf(`%s  profile=default  action=ready  steps=sound,say  afk=false
%s    step[1] sound  sound=success
%s    step[2] say  text="Ready!"

%s  profile=default  action=error  steps=sound  afk=false
%s    step[1] sound  sound=error

%s  profile=boss  action=ready  steps=sound  afk=true
%s    step[1] sound  sound=notification

%s  profile=default  action=ready  cooldown=skipped (30s)

`, ts, ts, ts, ts, ts, ts, ts, ts)

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
	if len(resp.Summary.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(resp.Summary.Profiles))
	}

	// Grand totals: 3 executions + 1 cooldown skip = 4.
	if resp.Summary.GrandTotal != 4 {
		t.Fatalf("expected grand_total 4, got %d", resp.Summary.GrandTotal)
	}
	if resp.Summary.GrandExec != 3 {
		t.Fatalf("expected grand_exec 3, got %d", resp.Summary.GrandExec)
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

	// Verify default profile.
	def := resp.Summary.Profiles[1]
	if def.Name != "default" {
		t.Fatalf("expected second profile 'default', got %q", def.Name)
	}
	if def.Total != 3 {
		t.Fatalf("expected default total 3, got %d", def.Total)
	}
	if len(def.Actions) != 2 {
		t.Fatalf("expected 2 actions for default, got %d", len(def.Actions))
	}

	// Verify hourly section.
	if len(resp.Hourly.Profiles) != 2 {
		t.Fatalf("expected 2 hourly profiles, got %d", len(resp.Hourly.Profiles))
	}
	if len(resp.Hourly.Hours) == 0 {
		t.Fatal("expected at least 1 hourly row")
	}
	if resp.Hourly.GrandTotal != 4 {
		t.Fatalf("expected hourly grand_total 4, got %d", resp.Hourly.GrandTotal)
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
