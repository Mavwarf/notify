//go:build darwin

package shell

import "testing"

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{`say "hello"`, `say \"hello\"`},
		{`"quoted"`, `\"quoted\"`},
		{"no quotes", "no quotes"},
		{"", ""},
		{`"""`, `\"\"\"`},
	}
	for _, tt := range tests {
		if got := EscapeAppleScript(tt.in); got != tt.want {
			t.Errorf("EscapeAppleScript(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
