//go:build windows

package toast

import (
	"fmt"
	"os/exec"

	"github.com/Mavwarf/notify/internal/shell"
)

// Show displays a Windows balloon-tip notification using PowerShell and
// System.Windows.Forms.NotifyIcon.
func Show(title, message string) error {
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.BalloonTipTitle = '%s'
$n.BalloonTipText = '%s'
$n.ShowBalloonTip(5000)
Start-Sleep -Seconds 3
$n.Dispose()
`, shell.EscapePowerShell(title), shell.EscapePowerShell(message))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast failed: %w\n%s", err, out)
	}
	return nil
}
