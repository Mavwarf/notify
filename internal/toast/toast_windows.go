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

// showScript returns the PowerShell script for displaying a modern Windows 10+
// toast notification using the ToastNotificationManager XML API.
//
// The toast includes an app logo icon and "via notify" attribution text.
// When desktop is non-nil, an action button is added that triggers the
// notify://switch?desktop=N protocol URI to switch virtual desktops.
func showScript(title, message, iconPath string, desktop *int) string {
	t := shell.EscapePowerShell(escapeXML(title))
	m := shell.EscapePowerShell(escapeXML(message))

	// Build the icon element if a path is available.
	iconElem := ""
	if iconPath != "" {
		// Convert backslashes to forward slashes for file:// URI.
		fileURI := "file:///" + strings.ReplaceAll(iconPath, `\`, "/")
		iconElem = fmt.Sprintf(`<image placement="appLogoOverride" src="%s"/>`,
			shell.EscapePowerShell(escapeXML(fileURI)))
	}

	// Build actions block for desktop switching button.
	actions := ""
	if desktop != nil {
		actions = fmt.Sprintf(
			`<actions><action content="Desktop %d" arguments="notify://switch?desktop=%d" activationType="protocol"/></actions>`,
			*desktop, *desktop)
	}

	return fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('<toast><visual><binding template="ToastGeneric">%s<text>%s</text><text>%s</text><text placement="attribution">via notify</text></binding></visual>%s</toast>')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)
`, iconElem, t, m, actions)
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
