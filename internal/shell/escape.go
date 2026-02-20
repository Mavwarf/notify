package shell

// EscapePowerShell doubles single quotes for safe embedding inside
// PowerShell single-quoted strings.
func EscapePowerShell(s string) string {
	var out []byte
	for _, b := range []byte(s) {
		if b == '\'' {
			out = append(out, '\'', '\'')
		} else {
			out = append(out, b)
		}
	}
	return string(out)
}
