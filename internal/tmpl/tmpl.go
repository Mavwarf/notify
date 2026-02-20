package tmpl

import "strings"

// Vars holds runtime values for template expansion.
type Vars struct {
	Profile     string
	Command     string
	Duration    string // compact: "2m15s"
	DurationSay string // spoken: "2 minutes and 15 seconds"
}

// Expand replaces template placeholders in s with runtime values.
// {profile} → profile name as-is, {Profile} → title-cased,
// {command} → wrapped command string,
// {duration} → compact elapsed time, {Duration} → spoken elapsed time.
func Expand(s string, v Vars) string {
	s = strings.ReplaceAll(s, "{Profile}", TitleCase(v.Profile))
	s = strings.ReplaceAll(s, "{profile}", v.Profile)
	s = strings.ReplaceAll(s, "{command}", v.Command)
	s = strings.ReplaceAll(s, "{Duration}", v.DurationSay)
	s = strings.ReplaceAll(s, "{duration}", v.Duration)
	return s
}

// TitleCase uppercases the first byte of s.
func TitleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
