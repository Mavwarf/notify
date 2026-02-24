package tmpl

import "strings"

// Vars holds runtime values for template expansion.
type Vars struct {
	Profile     string
	Command     string
	Duration    string // compact: "2m15s"
	DurationSay string // spoken: "2 minutes and 15 seconds"
	Time        string // compact: "15:04"
	TimeSay     string // spoken: "3:04 PM"
	Date        string // compact: "2006-01-02"
	DateSay     string // spoken: "January 2, 2006"
	Hostname    string
	Output      string // last N lines of wrapped command output
}

// Expand replaces template placeholders in s with runtime values.
// {profile} → profile name as-is, {Profile} → title-cased,
// {command} → wrapped command string,
// {duration} → compact elapsed time, {Duration} → spoken elapsed time.
//
// ORDER MATTERS: uppercase variants ({Duration}, {Time}, {Date}, {Profile})
// must be replaced before their lowercase counterparts. Otherwise
// "{Duration}" partially matches "{duration}" (a prefix), producing
// corrupted output. Do not reorder these replacements.
func Expand(s string, v Vars) string {
	s = strings.ReplaceAll(s, "{Profile}", TitleCase(v.Profile))
	s = strings.ReplaceAll(s, "{profile}", v.Profile)
	s = strings.ReplaceAll(s, "{command}", v.Command)
	s = strings.ReplaceAll(s, "{Duration}", v.DurationSay)
	s = strings.ReplaceAll(s, "{duration}", v.Duration)
	s = strings.ReplaceAll(s, "{Time}", v.TimeSay)
	s = strings.ReplaceAll(s, "{time}", v.Time)
	s = strings.ReplaceAll(s, "{Date}", v.DateSay)
	s = strings.ReplaceAll(s, "{date}", v.Date)
	s = strings.ReplaceAll(s, "{hostname}", v.Hostname)
	s = strings.ReplaceAll(s, "{output}", v.Output)
	return s
}

// TitleCase uppercases the first byte of s.
func TitleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
