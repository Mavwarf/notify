//go:build darwin

package shell

import "strings"

// EscapeAppleScript escapes double quotes for safe embedding inside
// AppleScript strings.
func EscapeAppleScript(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
