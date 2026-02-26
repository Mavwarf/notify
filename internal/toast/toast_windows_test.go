//go:build windows

package toast

import (
	"strings"
	"testing"
)

func TestShowScriptContainsTitle(t *testing.T) {
	s := showScript("Build Done", "finished in 5m")
	if !strings.Contains(s, "Build Done") {
		t.Errorf("script should contain title:\n%s", s)
	}
}

func TestShowScriptContainsMessage(t *testing.T) {
	s := showScript("Alert", "deploy complete")
	if !strings.Contains(s, "deploy complete") {
		t.Errorf("script should contain message:\n%s", s)
	}
}

func TestShowScriptEscapesTitleQuotes(t *testing.T) {
	s := showScript("it's ready", "done")
	if !strings.Contains(s, "it''s ready") {
		t.Errorf("script should escape title quotes:\n%s", s)
	}
}

func TestShowScriptEscapesMessageQuotes(t *testing.T) {
	s := showScript("Alert", "it's done")
	if !strings.Contains(s, "it''s done") {
		t.Errorf("script should escape message quotes:\n%s", s)
	}
}

func TestShowScriptLoadsFormsAssembly(t *testing.T) {
	s := showScript("T", "M")
	if !strings.Contains(s, "System.Windows.Forms") {
		t.Error("script should load System.Windows.Forms assembly")
	}
}

func TestShowScriptCreatesNotifyIcon(t *testing.T) {
	s := showScript("T", "M")
	if !strings.Contains(s, "NotifyIcon") {
		t.Error("script should create NotifyIcon")
	}
}

func TestShowScriptSetsBalloonTipTitle(t *testing.T) {
	s := showScript("My Title", "msg")
	if !strings.Contains(s, "$n.BalloonTipTitle = 'My Title'") {
		t.Errorf("script should set BalloonTipTitle:\n%s", s)
	}
}

func TestShowScriptSetsBalloonTipText(t *testing.T) {
	s := showScript("title", "My Message")
	if !strings.Contains(s, "$n.BalloonTipText = 'My Message'") {
		t.Errorf("script should set BalloonTipText:\n%s", s)
	}
}

func TestShowScriptDisposesIcon(t *testing.T) {
	s := showScript("T", "M")
	if !strings.Contains(s, "$n.Dispose()") {
		t.Error("script should dispose NotifyIcon")
	}
}

func TestShowScriptShowsBalloonTip(t *testing.T) {
	s := showScript("T", "M")
	if !strings.Contains(s, "ShowBalloonTip(5000)") {
		t.Error("script should show balloon tip for 5 seconds")
	}
}
