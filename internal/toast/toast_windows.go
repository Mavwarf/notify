//go:build windows

package toast

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Mavwarf/notify/internal/shell"
)

// escapeXML replaces XML-special characters so user content can be
// safely embedded inside XML text elements.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// powershellAUMID is the App User Model ID for Windows PowerShell. Using a
// registered AUMID ensures toast banners display reliably — Windows silently
// drops toasts from unregistered app IDs.
const powershellAUMID = `{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe`

// showScript returns the PowerShell script for displaying a modern Windows 10+
// toast notification using the ToastNotificationManager XML API.
//
// The toast includes an app logo icon and "via notify" attribution text.
// When desktop is non-nil, an action button is added that triggers the
// notify://switch?desktop=N protocol URI to switch virtual desktops.
func showScript(title, message, iconPath string, desktop *int) string {
	t := shell.EscapePowerShell(escapeXML(title))
	m := shell.EscapePowerShell(escapeXML(message))

	// Build toast XML piece by piece.
	var xml strings.Builder
	xml.WriteString(`<toast><visual><binding template="ToastGeneric">`)
	if iconPath != "" {
		fileURI := "file:///" + strings.ReplaceAll(iconPath, `\`, "/")
		fmt.Fprintf(&xml, `<image placement="appLogoOverride" src="%s"/>`,
			shell.EscapePowerShell(escapeXML(fileURI)))
	}
	fmt.Fprintf(&xml, `<text>%s</text>`, t)
	fmt.Fprintf(&xml, `<text>%s</text>`, m)
	xml.WriteString(`<text placement="attribution">via notify</text>`)
	xml.WriteString(`</binding></visual>`)
	if desktop != nil {
		fmt.Fprintf(&xml, `<actions><action content="Desktop %d" arguments="notify://switch?desktop=%d" activationType="protocol"/></actions>`,
			*desktop, *desktop)
	}
	xml.WriteString(`</toast>`)

	return fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('%s')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`, xml.String(), powershellAUMID)
}

// Show displays a Windows toast notification using the modern
// ToastNotificationManager XML API with app icon and attribution text.
// When desktop is non-nil, an action button switches virtual desktops.
func Show(title, message string, desktop *int) error {
	iconPath, _ := EnsureIcon() // best-effort; toast works without icon
	cmd := exec.Command("powershell", "-NoProfile", "-Command", showScript(title, message, iconPath, desktop))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}
