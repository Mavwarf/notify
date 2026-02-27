package plugin

import (
	"runtime"
	"strings"
	"testing"

	"github.com/Mavwarf/notify/internal/tmpl"
)

func fullVars() tmpl.Vars {
	return tmpl.Vars{
		Profile:       "myproject",
		Hostname:      "devbox",
		Time:          "14:30",
		Date:          "2026-02-27",
		Command:       "make build",
		Duration:      "2m15s",
		DurationSay:   "2 minutes and 15 seconds",
		Output:        "Build OK",
		ClaudeMessage: "Done coding",
		ClaudeHook:    "Stop",
		ClaudeJSON:    `{"hook":"Stop"}`,
	}
}

func TestBuildEnvAllFields(t *testing.T) {
	env := buildEnv("hello world", fullVars())
	want := map[string]string{
		"NOTIFY_PROFILE":       "myproject",
		"NOTIFY_HOSTNAME":      "devbox",
		"NOTIFY_TIME":          "14:30",
		"NOTIFY_DATE":          "2026-02-27",
		"NOTIFY_TEXT":          "hello world",
		"NOTIFY_COMMAND":       "make build",
		"NOTIFY_DURATION":      "2m15s",
		"NOTIFY_DURATION_SAY":  "2 minutes and 15 seconds",
		"NOTIFY_OUTPUT":        "Build OK",
		"NOTIFY_CLAUDE_MESSAGE": "Done coding",
		"NOTIFY_CLAUDE_HOOK":   "Stop",
		"NOTIFY_CLAUDE_JSON":   `{"hook":"Stop"}`,
	}

	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "NOTIFY_") {
			envMap[parts[0]] = parts[1]
		}
	}

	for k, v := range want {
		if got, ok := envMap[k]; !ok {
			t.Errorf("missing env var %s", k)
		} else if got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestBuildEnvEmptyOptionals(t *testing.T) {
	vars := tmpl.Vars{
		Profile:  "test",
		Hostname: "box",
		Time:     "09:00",
		Date:     "2026-01-01",
	}
	env := buildEnv("", vars)

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "NOTIFY_TEXT", "NOTIFY_COMMAND", "NOTIFY_DURATION",
			"NOTIFY_DURATION_SAY", "NOTIFY_OUTPUT",
			"NOTIFY_CLAUDE_MESSAGE", "NOTIFY_CLAUDE_HOOK", "NOTIFY_CLAUDE_JSON":
			t.Errorf("optional var %s should be absent when empty, got %q", parts[0], parts[1])
		}
	}

	// Required vars should still be present.
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for _, k := range []string{"NOTIFY_PROFILE", "NOTIFY_HOSTNAME", "NOTIFY_TIME", "NOTIFY_DATE"} {
		if _, ok := envMap[k]; !ok {
			t.Errorf("required var %s is missing", k)
		}
	}
}

func echoCmd() string {
	return "echo ok"
}

func failCmd() string {
	if runtime.GOOS == "windows" {
		return "echo FAIL>&2 && exit 1"
	}
	return "echo FAIL >&2; exit 1"
}

func sleepCmd() string {
	if runtime.GOOS == "windows" {
		return "ping -n 6 127.0.0.1 >nul"
	}
	return "sleep 5"
}

func TestRunSuccess(t *testing.T) {
	err := Run(echoCmd(), "", nil, tmpl.Vars{Profile: "test", Hostname: "h", Time: "0", Date: "0"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestRunFailure(t *testing.T) {
	err := Run(failCmd(), "", nil, tmpl.Vars{Profile: "test", Hostname: "h", Time: "0", Date: "0"})
	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("error should contain stderr output, got: %v", err)
	}
}

func TestRunTimeout(t *testing.T) {
	sec := 1
	err := Run(sleepCmd(), "", &sec, tmpl.Vars{Profile: "test", Hostname: "h", Time: "0", Date: "0"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

func TestRunDefaultTimeout(t *testing.T) {
	// nil timeout should use the 10s default — a quick command should succeed.
	err := Run(echoCmd(), "", nil, tmpl.Vars{Profile: "test", Hostname: "h", Time: "0", Date: "0"})
	if err != nil {
		t.Fatalf("expected success with default timeout, got: %v", err)
	}
}
