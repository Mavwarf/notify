//go:build windows

package toast

import (
	"fmt"
	"strings"
	"testing"
)

func TestShowScriptContainsTitle(t *testing.T) {
	s := showScript("Build Done", "finished in 5m", "", nil)
	if !strings.Contains(s, "Build Done") {
		t.Errorf("script should contain title:\n%s", s)
	}
}

func TestShowScriptContainsMessage(t *testing.T) {
	s := showScript("Alert", "deploy complete", "", nil)
	if !strings.Contains(s, "deploy complete") {
		t.Errorf("script should contain message:\n%s", s)
	}
}

func TestShowScriptEscapesTitleQuotes(t *testing.T) {
	s := showScript("it's ready", "done", "", nil)
	if !strings.Contains(s, "it&apos;s ready") {
		t.Errorf("script should XML-escape title quotes:\n%s", s)
	}
}

func TestShowScriptEscapesMessageQuotes(t *testing.T) {
	s := showScript("Alert", "it's done", "", nil)
	if !strings.Contains(s, "it&apos;s done") {
		t.Errorf("script should XML-escape message quotes:\n%s", s)
	}
}

func TestShowScriptLoadsWinRT(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if !strings.Contains(s, "Windows.UI.Notifications.ToastNotificationManager") {
		t.Error("script should load ToastNotificationManager WinRT type")
	}
}

func TestShowScriptUsesXmlDocument(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if !strings.Contains(s, "Windows.Data.Xml.Dom.XmlDocument") {
		t.Error("script should use XmlDocument")
	}
}

func TestShowScriptContainsToastGenericTemplate(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if !strings.Contains(s, "ToastGeneric") {
		t.Error("script should use ToastGeneric binding template")
	}
}

func TestShowScriptCreatesNotifier(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if !strings.Contains(s, "CreateToastNotifier") {
		t.Error("script should create toast notifier")
	}
	if !strings.Contains(s, "WindowsPowerShell") {
		t.Error("script should use PowerShell AUMID for reliable toast display")
	}
}

func TestShowScriptNilDesktopNoActions(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if strings.Contains(s, "<actions>") {
		t.Error("nil desktop should not include actions block")
	}
	if strings.Contains(s, "notify://switch") {
		t.Error("nil desktop should not include protocol URI")
	}
}

func TestShowScriptDesktopActionButton(t *testing.T) {
	d := 2
	s := showScript("T", "M", "", &d)
	if !strings.Contains(s, `<actions>`) {
		t.Errorf("desktop should include actions block:\n%s", s)
	}
	if !strings.Contains(s, `content="Desktop 2"`) {
		t.Errorf("desktop=2 should have button labeled Desktop 2:\n%s", s)
	}
	if !strings.Contains(s, `arguments="notify://switch?desktop=2"`) {
		t.Errorf("desktop=2 button should have protocol URI:\n%s", s)
	}
	if !strings.Contains(s, `activationType="protocol"`) {
		t.Errorf("desktop button should have activationType=protocol:\n%s", s)
	}
}

func TestShowScriptDesktopNoToastLevelActivation(t *testing.T) {
	d := 2
	s := showScript("T", "M", "", &d)
	// activationType should only appear inside <action>, not on <toast>
	toastTag := "<toast>"
	if !strings.Contains(s, toastTag) {
		t.Error("toast element should have no activation attributes")
	}
	if strings.Contains(s, `<toast activationType`) {
		t.Error("toast element should not have activationType attribute")
	}
}

func TestShowScriptEscapesXMLChars(t *testing.T) {
	s := showScript("A & B", "x < y > z", "", nil)
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
		s := showScript("T", "M", "", &d)
		expected := fmt.Sprintf(`arguments="notify://switch?desktop=%d"`, d)
		if !strings.Contains(s, expected) {
			t.Errorf("desktop=%d: expected %s in script:\n%s", d, expected, s)
		}
		label := fmt.Sprintf(`content="Desktop %d"`, d)
		if !strings.Contains(s, label) {
			t.Errorf("desktop=%d: expected button label %s:\n%s", d, label, s)
		}
	}
}

func TestShowScriptAttribution(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if !strings.Contains(s, `placement="attribution"`) {
		t.Error("script should contain attribution placement")
	}
	if !strings.Contains(s, "via notify") {
		t.Error("script should contain 'via notify' attribution text")
	}
}

func TestShowScriptAppLogoOverride(t *testing.T) {
	s := showScript("T", "M", `C:\Users\test\AppData\Roaming\notify\icon.png`, nil)
	if !strings.Contains(s, `placement="appLogoOverride"`) {
		t.Errorf("script should contain appLogoOverride placement:\n%s", s)
	}
	if !strings.Contains(s, "file:///C:/Users/test/AppData/Roaming/notify/icon.png") {
		t.Errorf("script should contain file:// URI with forward slashes:\n%s", s)
	}
}

func TestShowScriptNoIconPath(t *testing.T) {
	s := showScript("T", "M", "", nil)
	if strings.Contains(s, "appLogoOverride") {
		t.Error("empty icon path should not include appLogoOverride element")
	}
}
