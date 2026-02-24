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
		{"time var", "at {time}", Vars{Time: "14:30"}, "at 14:30"},
		{"Time spoken", "at {Time}", Vars{TimeSay: "2:30 PM"}, "at 2:30 PM"},
		{"date var", "on {date}", Vars{Date: "2026-02-22"}, "on 2026-02-22"},
		{"Date spoken", "on {Date}", Vars{DateSay: "February 22, 2026"}, "on February 22, 2026"},
		{"hostname var", "from {hostname}", Vars{Hostname: "mypc"}, "from mypc"},
		{"empty time", "{time}", Vars{}, ""},
		{"empty Time", "{Time}", Vars{}, ""},
		{"empty date", "{date}", Vars{}, ""},
		{"empty Date", "{Date}", Vars{}, ""},
		{"empty hostname", "{hostname}", Vars{}, ""},
		{"all new vars", "{date} {time} {hostname}", Vars{Time: "09:00", Date: "2026-01-01", Hostname: "srv"}, "2026-01-01 09:00 srv"},
		{"spoken vars", "{Date} at {Time}", Vars{TimeSay: "9:00 AM", DateSay: "January 1, 2026"}, "January 1, 2026 at 9:00 AM"},
		{"output var", "result:\n{output}", Vars{Output: "3 passed, 1 failed"}, "result:\n3 passed, 1 failed"},
		{"empty output", "{output}", Vars{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Expand(tt.s, tt.vars); got != tt.want {
				t.Errorf("Expand(%q, %+v) = %q, want %q", tt.s, tt.vars, got, tt.want)
			}
		})
	}
}
