package tmpl

import "testing"

func TestTitleCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"boss", "Boss"},
		{"Boss", "Boss"},
		{"default", "Default"},
		{"a", "A"},
	}
	for _, tt := range tests {
		if got := TitleCase(tt.in); got != tt.want {
			t.Errorf("TitleCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		s, profile, want string
	}{
		{"Hello", "boss", "Hello"},
		{"{profile} ready", "boss", "boss ready"},
		{"{Profile} ready", "boss", "Boss ready"},
		{"{Profile}: {profile}", "dev", "Dev: dev"},
		{"no placeholders", "x", "no placeholders"},
		{"{profile}", "", ""},
		{"{Profile}", "", ""},
	}
	for _, tt := range tests {
		if got := Expand(tt.s, tt.profile); got != tt.want {
			t.Errorf("Expand(%q, %q) = %q, want %q", tt.s, tt.profile, got, tt.want)
		}
	}
}
