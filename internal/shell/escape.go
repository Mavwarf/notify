package shell

import "strings"

// EscapePowerShell doubles single quotes for safe embedding inside
// PowerShell single-quoted strings.
func EscapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
