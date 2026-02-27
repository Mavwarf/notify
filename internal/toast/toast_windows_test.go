//go:build windows

package toast

import (
	"fmt"
	"strings"
	"testing"
)

func TestShowScriptContainsTitle(t *testing.T) {
	s := showScript("Build Done", "finished in 5m", nil)
	if !strings.Contains(s, "Build Done") {
		t.Errorf("script should contain title:\n%s", s)
	}
}

func TestShowScriptContainsMessage(t *testing.T) {
	s := showScript("Alert", "deploy complete", nil)
	if !strings.Contains(s, "deploy complete") {
		t.Errorf("script should contain message:\n%s", s)
	}
}

func TestShowScriptEscapesTitleQuotes(t *testing.T) {
	s := showScript("it's ready", "done", nil)
	if !strings.Contains(s, "it&apos;s ready") {
		t.Errorf("script should XML-escape title quotes:\n%s", s)
	}
}

func TestShowScriptEscapesMessageQuotes(t *testing.T) {
	s := showScript("Alert", "it's done", nil)
	if !strings.Contains(s, "it&apos;s done") {
		t.Errorf("script should XML-escape message quotes:\n%s", s)
	}
}

func TestShowScriptLoadsWinRT(t *testing.T) {
	s := showScript("T", "M", nil)
	if !strings.Contains(s, "Windows.UI.Notifications.ToastNotificationManager") {
		t.Error("script should load ToastNotificationManager WinRT type")
	}
}

func TestShowScriptUsesXmlDocument(t *testing.T) {
	s := showScript("T", "M", nil)
	if !strings.Contains(s, "Windows.Data.Xml.Dom.XmlDocument") {
		t.Error("script should use XmlDocument")
	}
}

func TestShowScriptContainsToastGenericTemplate(t *testing.T) {
	s := showScript("T", "M", nil)
	if !strings.Contains(s, "ToastGeneric") {
		t.Error("script should use ToastGeneric binding template")
	}
}

func TestShowScriptCreatesNotifier(t *testing.T) {
	s := showScript("T", "M", nil)
	if !strings.Contains(s, "CreateToastNotifier") {
		t.Error("script should create toast notifier")
	}
}

func TestShowScriptNilDesktopNoProtocol(t *testing.T) {
	s := showScript("T", "M", nil)
	if strings.Contains(s, "activationType") {
		t.Error("nil desktop should not include activationType")
	}
	if strings.Contains(s, "notify://switch") {
		t.Error("nil desktop should not include protocol URI")
	}
}

func TestShowScriptDesktopProtocolActivation(t *testing.T) {
	d := 2
	s := showScript("T", "M", &d)
	if !strings.Contains(s, `activationType="protocol"`) {
		t.Errorf("desktop should set activationType=protocol:\n%s", s)
	}
	if !strings.Contains(s, `launch="notify://switch?desktop=2"`) {
		t.Errorf("desktop=2 should set launch URI:\n%s", s)
	}
}

func TestShowScriptEscapesXMLChars(t *testing.T) {
	s := showScript("A & B", "x < y > z", nil)
	if strings.Contains(s, "A & B") {
		t.Error("ampersand should be XML-escaped")
	}
	if !strings.Contains(s, "A &amp; B") {
		t.Errorf("title should contain escaped ampersand:\n%s", s)
	}
	if !strings.Contains(s, "x &lt; y &gt; z") {
		t.Errorf("message should contain escaped angle brackets:\n%s", s)
	}
}

func TestShowScriptDesktopOtherValues(t *testing.T) {
	for _, d := range []int{1, 3, 4} {
		s := showScript("T", "M", &d)
		expected := fmt.Sprintf(`launch="notify://switch?desktop=%d"`, d)
		if !strings.Contains(s, expected) {
			t.Errorf("desktop=%d: expected %s in script:\n%s", d, expected, s)
		}
	}
}
