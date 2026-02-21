package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultAFKThreshold is the default idle-time threshold in seconds.
const DefaultAFKThreshold = 300

// DefaultVolume is the default playback volume (0-100).
const DefaultVolume = 100

// Credentials holds secret values for remote notification actions.
type Credentials struct {
	DiscordWebhook string `json:"discord_webhook,omitempty"`
}

// Options holds global settings parsed from the "config" key.
type Options struct {
	AFKThresholdSeconds int         `json:"afk_threshold_seconds,omitempty"`
	DefaultVolume       int         `json:"default_volume,omitempty"`
	Log                 bool        `json:"log,omitempty"`
	Cooldown            bool        `json:"cooldown,omitempty"`
	CooldownSeconds     int         `json:"cooldown_seconds,omitempty"`
	Credentials         Credentials `json:"credentials,omitempty"`
}

// Config holds the top-level configuration: global options and profiles.
type Config struct {
	Options  Options            `json:"config"`
	Profiles map[string]Profile `json:"profiles"`
}

// UnmarshalJSON sets defaults then decodes the JSON structure.
// Go's json.Unmarshal merges into existing struct fields, so only
// values present in JSON override the defaults.
func (c *Config) UnmarshalJSON(data []byte) error {
	c.Options.AFKThresholdSeconds = DefaultAFKThreshold
	c.Options.DefaultVolume = DefaultVolume
	type Alias Config
	return json.Unmarshal(data, (*Alias)(c))
}

// Profile maps action names to actions.
type Profile map[string]Action

// Action holds an ordered list of steps to execute.
type Action struct {
	CooldownSeconds int    `json:"cooldown_seconds,omitempty"`
	Steps           []Step `json:"steps"`
}

// Step is a single unit of work within an action.
type Step struct {
	Type    string `json:"type"`              // "sound" | "say" | "toast" | "discord"
	Sound   string `json:"sound,omitempty"`   // type=sound
	Text    string `json:"text,omitempty"`    // type=say
	Title   string `json:"title,omitempty"`   // type=toast
	Message string `json:"message,omitempty"` // type=toast
	Volume  *int   `json:"volume,omitempty"`  // per-step override, nil = use default
	When    string `json:"when,omitempty"`    // "afk" | "present" | "" (always)
}

// Load reads and parses a config file. It tries, in order:
//  1. explicitPath (if non-empty)
//  2. notify-config.json next to the running binary
//  3. ~/.config/notify/notify-config.json
func Load(explicitPath string) (Config, error) {
	if explicitPath != "" {
		return readConfig(explicitPath)
	}

	// Next to binary
	exe, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(exe), "notify-config.json")
		if _, err := os.Stat(p); err == nil {
			return readConfig(p)
		}
	}

	// User config directory
	home, err := os.UserHomeDir()
	if err == nil {
		var p string
		if runtime.GOOS == "windows" {
			p = filepath.Join(home, "AppData", "Roaming", "notify", "notify-config.json")
		} else {
			p = filepath.Join(home, ".config", "notify", "notify-config.json")
		}
		if _, err := os.Stat(p); err == nil {
			return readConfig(p)
		}
	}

	return Config{}, fmt.Errorf("no notify-config.json found (use --config to specify a path)")
}

// Resolve looks up an action by profile and action name.
// Falls back to the "default" profile if the requested profile
// doesn't contain the action.
func Resolve(cfg Config, profile, action string) (*Action, error) {
	if p, ok := cfg.Profiles[profile]; ok {
		if a, ok := p[action]; ok {
			return &a, nil
		}
	}
	if profile != "default" {
		if p, ok := cfg.Profiles["default"]; ok {
			if a, ok := p[action]; ok {
				return &a, nil
			}
		}
	}
	return nil, fmt.Errorf("action %q not found in profile %q or default", action, profile)
}

func readConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
