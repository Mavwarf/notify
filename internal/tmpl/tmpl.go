package tmpl

import "strings"

// Expand replaces template placeholders in s with runtime values.
// {profile} → profile name as-is, {Profile} → title-cased.
func Expand(s, profile string) string {
	s = strings.ReplaceAll(s, "{Profile}", TitleCase(profile))
	s = strings.ReplaceAll(s, "{profile}", profile)
	return s
}

// TitleCase uppercases the first byte of s.
func TitleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
