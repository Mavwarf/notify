package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Mavwarf/notify/internal/paths"
)

// DefaultAFKThreshold is the default idle-time threshold in seconds.
const DefaultAFKThreshold = 300

// DefaultVolume is the default playback volume (0-100).
const DefaultVolume = 100

// Credentials holds secret values for remote notification actions.
type Credentials struct {
	DiscordWebhook string `json:"discord_webhook,omitempty"`
	TelegramToken  string `json:"telegram_token,omitempty"`
	TelegramChatID string `json:"telegram_chat_id,omitempty"`
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
	Type    string `json:"type"`              // "sound" | "say" | "toast" | "discord" | "discord_voice" | "telegram" | "telegram_audio"
	Sound   string `json:"sound,omitempty"`   // type=sound
	Text    string `json:"text,omitempty"`    // type=say, discord, discord_voice, telegram, telegram_audio
	Title   string `json:"title,omitempty"`   // type=toast
	Message string `json:"message,omitempty"` // type=toast
	Volume  *int   `json:"volume,omitempty"`  // per-step override, nil = use default
	When    string `json:"when,omitempty"`    // "" | "afk" | "present" | "run" | "direct" | "hours:X-Y"
}

// validStepTypes is the set of recognized step types.
var validStepTypes = map[string]bool{
	"sound": true, "say": true, "toast": true, "discord": true, "discord_voice": true, "telegram": true, "telegram_audio": true,
}

// Validate checks a parsed Config for common mistakes and returns a
// multi-line error listing all problems found, or nil if valid.
func Validate(cfg Config) error {
	var errs []string

	// Global options.
	if cfg.Options.DefaultVolume < 0 || cfg.Options.DefaultVolume > 100 {
		errs = append(errs, fmt.Sprintf("config: default_volume %d out of range 0-100", cfg.Options.DefaultVolume))
	}
	if cfg.Options.AFKThresholdSeconds < 0 {
		errs = append(errs, fmt.Sprintf("config: afk_threshold_seconds %d must not be negative", cfg.Options.AFKThresholdSeconds))
	}
	if cfg.Options.CooldownSeconds < 0 {
		errs = append(errs, fmt.Sprintf("config: cooldown_seconds %d must not be negative", cfg.Options.CooldownSeconds))
	}

	// Profiles and steps.
	for pName, profile := range cfg.Profiles {
		for aName, action := range profile {
			prefix := fmt.Sprintf("profiles.%s.%s", pName, aName)
			if len(action.Steps) == 0 {
				errs = append(errs, fmt.Sprintf("%s: action has no steps", prefix))
			}
			if action.CooldownSeconds < 0 {
				errs = append(errs, fmt.Sprintf("%s: cooldown_seconds %d must not be negative", prefix, action.CooldownSeconds))
			}
			for i, s := range action.Steps {
				sp := fmt.Sprintf("%s.steps[%d]", prefix, i)
				if !validStepTypes[s.Type] {
					errs = append(errs, fmt.Sprintf("%s: unknown type %q", sp, s.Type))
				}
				if err := validateWhen(s.When); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", sp, err))
				}
				if s.Volume != nil && (*s.Volume < 0 || *s.Volume > 100) {
					errs = append(errs, fmt.Sprintf("%s: volume %d out of range 0-100", sp, *s.Volume))
				}
				// Required fields per type.
				switch s.Type {
				case "sound":
					if s.Sound == "" {
						errs = append(errs, fmt.Sprintf("%s: sound step requires \"sound\" field", sp))
					}
				case "say":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: say step requires \"text\" field", sp))
					}
				case "toast":
					if s.Message == "" {
						errs = append(errs, fmt.Sprintf("%s: toast step requires \"message\" field", sp))
					}
				case "discord":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: discord step requires \"text\" field", sp))
					}
					if cfg.Options.Credentials.DiscordWebhook == "" {
						errs = append(errs, fmt.Sprintf("%s: discord step requires credentials.discord_webhook", sp))
					}
				case "discord_voice":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: discord_voice step requires \"text\" field", sp))
					}
					if cfg.Options.Credentials.DiscordWebhook == "" {
						errs = append(errs, fmt.Sprintf("%s: discord_voice step requires credentials.discord_webhook", sp))
					}
				case "telegram":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: telegram step requires \"text\" field", sp))
					}
					if cfg.Options.Credentials.TelegramToken == "" || cfg.Options.Credentials.TelegramChatID == "" {
						errs = append(errs, fmt.Sprintf("%s: telegram step requires credentials.telegram_token and telegram_chat_id", sp))
					}
				case "telegram_audio":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: telegram_audio step requires \"text\" field", sp))
					}
					if cfg.Options.Credentials.TelegramToken == "" || cfg.Options.Credentials.TelegramChatID == "" {
						errs = append(errs, fmt.Sprintf("%s: telegram_audio step requires credentials.telegram_token and telegram_chat_id", sp))
					}
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("config validation:\n  %s", strings.Join(errs, "\n  "))
}

// validateWhen checks that a when condition string is recognized.
func validateWhen(when string) error {
	switch when {
	case "", "afk", "present", "run", "direct":
		return nil
	default:
		if strings.HasPrefix(when, "hours:") {
			return validateHoursSpec(when[6:])
		}
		return fmt.Errorf("unknown when condition %q", when)
	}
}

// validateHoursSpec checks that a hours spec like "8-22" is well-formed.
func validateHoursSpec(spec string) error {
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid hours spec %q (expected X-Y)", spec)
	}
	start, err1 := strconv.Atoi(parts[0])
	end, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || start < 0 || start > 23 || end < 0 || end > 23 {
		return fmt.Errorf("invalid hours spec %q (hours must be 0-23)", spec)
	}
	return nil
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
		p := filepath.Join(filepath.Dir(exe), paths.ConfigFileName)
		if _, err := os.Stat(p); err == nil {
			return readConfig(p)
		}
	}

	// User config directory
	p := filepath.Join(paths.DataDir(), paths.ConfigFileName)
	if _, err := os.Stat(p); err == nil {
		return readConfig(p)
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
