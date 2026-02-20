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
		name string
		s    string
		vars Vars
		want string
	}{
		{"no placeholders", "Hello", Vars{Profile: "boss"}, "Hello"},
		{"profile lower", "{profile} ready", Vars{Profile: "boss"}, "boss ready"},
		{"profile title", "{Profile} ready", Vars{Profile: "boss"}, "Boss ready"},
		{"both profiles", "{Profile}: {profile}", Vars{Profile: "dev"}, "Dev: dev"},
		{"empty profile", "{profile}", Vars{}, ""},
		{"empty Profile", "{Profile}", Vars{}, ""},
		{"command var", "{command} done", Vars{Command: "make build"}, "make build done"},
		{"duration compact", "took {duration}", Vars{Duration: "3s"}, "took 3s"},
		{"duration spoken", "took {Duration}", Vars{DurationSay: "3 seconds"}, "took 3 seconds"},
		{"both durations", "{duration} ({Duration})", Vars{Duration: "2m15s", DurationSay: "2 minutes and 15 seconds"}, "2m15s (2 minutes and 15 seconds)"},
		{"all vars", "{command} in {Duration} for {Profile}", Vars{Profile: "boss", Command: "cargo test", DurationSay: "2 minutes and 15 seconds"}, "cargo test in 2 minutes and 15 seconds for Boss"},
		{"empty command", "{command}", Vars{}, ""},
		{"empty duration", "{duration}", Vars{}, ""},
		{"empty Duration", "{Duration}", Vars{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Expand(tt.s, tt.vars); got != tt.want {
				t.Errorf("Expand(%q, %+v) = %q, want %q", tt.s, tt.vars, got, tt.want)
			}
		})
	}
}
