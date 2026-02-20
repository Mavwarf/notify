package shell

import "testing"

func TestEscapePowerShell(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"it's done", "it''s done"},
		{"'quoted'", "''quoted''"},
		{"no quotes", "no quotes"},
		{"", ""},
		{"'''", "''''''"},
	}
	for _, tt := range tests {
		if got := EscapePowerShell(tt.in); got != tt.want {
			t.Errorf("EscapePowerShell(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
