//go:build windows

package toast

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

// showScript returns the PowerShell script for displaying a modern Windows 10+
// toast notification using the ToastNotificationManager XML API.
//
// When desktop is non-nil, the toast includes a protocol activation URI
// (notify://switch?desktop=N) so clicking it switches virtual desktops.
func showScript(title, message string, desktop *int) string {
	t := shell.EscapePowerShell(title)
	m := shell.EscapePowerShell(message)

	// Build activation attributes for the toast element.
	activation := ""
	if desktop != nil {
		activation = fmt.Sprintf(` activationType="protocol" launch="notify://switch?desktop=%d"`, *desktop)
	}

	return fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('<toast%s><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('notify').Show($toast)
`, activation, t, m)
}

// Show displays a Windows toast notification using the modern
// ToastNotificationManager XML API. When desktop is non-nil, the toast
// includes a protocol activation URI for virtual desktop switching.
func Show(title, message string, desktop *int) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", showScript(title, message, desktop))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}
