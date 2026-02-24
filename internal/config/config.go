package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	SlackWebhook   string `json:"slack_webhook,omitempty"`
	TelegramToken  string `json:"telegram_token,omitempty"`
	TelegramChatID string `json:"telegram_chat_id,omitempty"`
}

// Options holds global settings parsed from the "config" key.
type Options struct {
	AFKThresholdSeconds int               `json:"afk_threshold_seconds,omitempty"`
	DefaultVolume       int               `json:"default_volume,omitempty"`
	Log                 bool              `json:"log,omitempty"`
	Echo                bool              `json:"echo,omitempty"`
	Cooldown            bool              `json:"cooldown,omitempty"`
	CooldownSeconds     int               `json:"cooldown_seconds,omitempty"`
	ExitCodes           map[string]string `json:"exit_codes,omitempty"`
	Credentials         Credentials       `json:"credentials,omitempty"`
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

// MatchRule defines conditions for automatic profile selection.
// All non-empty fields must match (AND logic). Dir is a substring
// check against the working directory (forward-slash normalized).
// Env is a KEY=VALUE check against an environment variable.
type MatchRule struct {
	Dir string `json:"dir,omitempty"`
	Env string `json:"env,omitempty"`
}

// Profile holds a set of actions and an optional parent profile name.
// When "extends" is set, the profile inherits all actions from the
// parent, with its own actions taking priority on conflicts.
// Aliases provide shorthand names for the profile.
// Credentials override global credentials field-by-field (nil = use global only).
// Match defines conditions for automatic profile selection (nil = never auto-selected).
type Profile struct {
	Extends     string            `json:"-"`
	Aliases     []string          `json:"-"`
	Credentials *Credentials      `json:"-"`
	Match       *MatchRule        `json:"-"`
	Actions     map[string]Action `json:"-"`
}

// UnmarshalJSON extracts the optional "extends" key and parses all
// remaining keys as actions. This keeps the JSON format flat:
//
//	{ "extends": "default", "ready": { "steps": [...] } }
func (p *Profile) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if ext, ok := raw["extends"]; ok {
		if err := json.Unmarshal(ext, &p.Extends); err != nil {
			return fmt.Errorf("extends: %w", err)
		}
		delete(raw, "extends")
	}
	if al, ok := raw["aliases"]; ok {
		if err := json.Unmarshal(al, &p.Aliases); err != nil {
			return fmt.Errorf("aliases: %w", err)
		}
		delete(raw, "aliases")
	}
	if cr, ok := raw["credentials"]; ok {
		var creds Credentials
		if err := json.Unmarshal(cr, &creds); err != nil {
			return fmt.Errorf("credentials: %w", err)
		}
		p.Credentials = &creds
		delete(raw, "credentials")
	}
	if m, ok := raw["match"]; ok {
		var rule MatchRule
		if err := json.Unmarshal(m, &rule); err != nil {
			return fmt.Errorf("match: %w", err)
		}
		p.Match = &rule
		delete(raw, "match")
	}
	p.Actions = make(map[string]Action, len(raw))
	for k, v := range raw {
		var a Action
		if err := json.Unmarshal(v, &a); err != nil {
			return fmt.Errorf("action %q: %w", k, err)
		}
		p.Actions[k] = a
	}
	return nil
}

// Action holds an ordered list of steps to execute.
type Action struct {
	CooldownSeconds int    `json:"cooldown_seconds,omitempty"`
	Steps           []Step `json:"steps"`
}

// Step is a single unit of work within an action.
type Step struct {
	Type    string            `json:"type"`              // "sound" | "say" | "toast" | "discord" | "discord_voice" | "slack" | "telegram" | "telegram_audio" | "telegram_voice" | "webhook"
	Sound   string            `json:"sound,omitempty"`   // type=sound
	Text    string            `json:"text,omitempty"`    // type=say, discord, discord_voice, slack, telegram, telegram_audio, webhook
	Title   string            `json:"title,omitempty"`   // type=toast
	Message string            `json:"message,omitempty"` // type=toast
	URL     string            `json:"url,omitempty"`     // type=webhook
	Headers map[string]string `json:"headers,omitempty"` // type=webhook
	Volume  *int              `json:"volume,omitempty"`  // per-step override, nil = use default
	When    string            `json:"when,omitempty"`    // "" | "never" | "afk" | "present" | "run" | "direct" | "hours:X-Y"
}

// validStepTypes is the set of recognized step types.
var validStepTypes = map[string]bool{
	"sound": true, "say": true, "toast": true, "discord": true, "discord_voice": true, "slack": true, "telegram": true, "telegram_audio": true, "telegram_voice": true, "webhook": true,
}

// builtinSounds is the set of built-in sound names. Kept in sync with
// audio.Sounds — used here to distinguish built-in names from file paths
// without importing the audio package.
var builtinSounds = map[string]bool{
	"warning": true, "success": true, "error": true, "info": true,
	"alert": true, "notification": true, "blip": true,
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
	for k, v := range cfg.Options.ExitCodes {
		if _, err := strconv.Atoi(k); err != nil {
			errs = append(errs, fmt.Sprintf("config: exit_codes key %q is not a valid integer", k))
		}
		if v == "" {
			errs = append(errs, fmt.Sprintf("config: exit_codes[%q] action must not be empty", k))
		}
	}

	// Alias validation.
	aliasOwner := map[string]string{} // alias → profile name
	for pName, profile := range cfg.Profiles {
		for _, alias := range profile.Aliases {
			if _, ok := cfg.Profiles[alias]; ok {
				errs = append(errs, fmt.Sprintf("profiles.%s: alias %q shadows an existing profile name", pName, alias))
			}
			if prev, ok := aliasOwner[alias]; ok {
				errs = append(errs, fmt.Sprintf("profiles.%s: alias %q already claimed by profile %q", pName, alias, prev))
			}
			aliasOwner[alias] = pName
		}
	}

	// Match rule validation.
	for pName, profile := range cfg.Profiles {
		if profile.Match != nil {
			if profile.Match.Dir == "" && profile.Match.Env == "" {
				errs = append(errs, fmt.Sprintf("profiles.%s: match rule must have at least one condition (dir or env)", pName))
			}
			if profile.Match.Env != "" {
				parts := strings.SplitN(profile.Match.Env, "=", 2)
				if len(parts) < 2 || parts[0] == "" {
					errs = append(errs, fmt.Sprintf("profiles.%s: match env must be KEY=VALUE (got %q)", pName, profile.Match.Env))
				}
			}
		}
	}

	// Profiles and steps.
	for pName, profile := range cfg.Profiles {
		creds := MergeCredentials(cfg.Options.Credentials, profile.Credentials)
		for aName, action := range profile.Actions {
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
				case "discord", "discord_voice":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: %s step requires \"text\" field", sp, s.Type))
					}
					if creds.DiscordWebhook == "" {
						errs = append(errs, fmt.Sprintf("%s: %s step requires credentials.discord_webhook", sp, s.Type))
					}
				case "slack":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: slack step requires \"text\" field", sp))
					}
					if creds.SlackWebhook == "" {
						errs = append(errs, fmt.Sprintf("%s: slack step requires credentials.slack_webhook", sp))
					}
				case "telegram", "telegram_audio", "telegram_voice":
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: %s step requires \"text\" field", sp, s.Type))
					}
					if creds.TelegramToken == "" || creds.TelegramChatID == "" {
						errs = append(errs, fmt.Sprintf("%s: %s step requires credentials.telegram_token and telegram_chat_id", sp, s.Type))
					}
				case "webhook":
					if s.URL == "" {
						errs = append(errs, fmt.Sprintf("%s: webhook step requires \"url\" field", sp))
					}
					if s.Text == "" {
						errs = append(errs, fmt.Sprintf("%s: webhook step requires \"text\" field", sp))
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
	case "", "afk", "present", "run", "direct", "never":
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

// FindPath resolves the config file path using the same resolution order
// as Load. Returns the resolved path or an error if no config file is found.
func FindPath(explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	// Next to binary
	exe, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(exe), paths.ConfigFileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// User config directory
	p := filepath.Join(paths.DataDir(), paths.ConfigFileName)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("no notify-config.json found (use --config to specify a path)")
}

// Load reads and parses a config file. It tries, in order:
//  1. explicitPath (if non-empty)
//  2. notify-config.json next to the running binary
//  3. ~/.config/notify/notify-config.json
func Load(explicitPath string) (Config, error) {
	p, err := FindPath(explicitPath)
	if err != nil {
		return Config{}, err
	}
	return readConfig(p)
}

// Resolve looks up an action by profile and action name.
// Returns the canonical profile name (resolving aliases), the action,
// and an error. Checks direct match first, then alias match, then
// falls back to the "default" profile.
func Resolve(cfg Config, profile, action string) (string, *Action, error) {
	// Direct match.
	if p, ok := cfg.Profiles[profile]; ok {
		if a, ok := p.Actions[action]; ok {
			return profile, &a, nil
		}
	}
	// Alias match.
	canonical := profile
	for pName, p := range cfg.Profiles {
		for _, alias := range p.Aliases {
			if alias == profile {
				if a, ok := p.Actions[action]; ok {
					return pName, &a, nil
				}
				canonical = pName
				break
			}
		}
	}
	// Default fallback.
	if canonical != "default" {
		if p, ok := cfg.Profiles["default"]; ok {
			if a, ok := p.Actions[action]; ok {
				return canonical, &a, nil
			}
		}
	}
	return "", nil, fmt.Errorf("action %q not found in profile %q or default", action, canonical)
}

// MatchProfile returns the first profile whose match rule is satisfied
// by the given working directory, or "default" if none match. Profiles
// are checked alphabetically for deterministic tiebreaking. Profiles
// without a match rule are skipped.
func MatchProfile(cfg Config, dir string) string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		rule := cfg.Profiles[name].Match
		if rule == nil {
			continue
		}
		if rule.Dir != "" {
			if !strings.Contains(filepath.ToSlash(dir), rule.Dir) {
				continue
			}
		}
		if rule.Env != "" {
			parts := strings.SplitN(rule.Env, "=", 2)
			if len(parts) == 2 && os.Getenv(parts[0]) != parts[1] {
				continue
			}
		}
		return name
	}
	return "default"
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
	if err := resolveInheritance(&cfg); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	expandEnvCredentials(&cfg)
	resolveSoundPaths(&cfg, filepath.Dir(path))
	return cfg, nil
}

// resolveInheritance flattens profile inheritance chains. For each
// profile with an "extends" field, parent actions are merged in
// (child actions take priority). Detects circular chains and unknown parents.
func resolveInheritance(cfg *Config) error {
	resolved := make(map[string]bool)
	resolving := make(map[string]bool)

	var resolve func(name string) error
	resolve = func(name string) error {
		if resolved[name] {
			return nil
		}
		if resolving[name] {
			return fmt.Errorf("circular extends chain involving %q", name)
		}

		profile, ok := cfg.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q not found", name)
		}

		if profile.Extends == "" {
			resolved[name] = true
			return nil
		}

		parent := profile.Extends
		if _, ok := cfg.Profiles[parent]; !ok {
			return fmt.Errorf("profile %q extends unknown profile %q", name, parent)
		}

		resolving[name] = true
		if err := resolve(parent); err != nil {
			return err
		}
		delete(resolving, name)

		// Merge parent credentials into child (child wins on conflict).
		parentProfile := cfg.Profiles[parent]
		if profile.Credentials == nil {
			profile.Credentials = parentProfile.Credentials
		} else if parentProfile.Credentials != nil {
			merged := MergeCredentials(*parentProfile.Credentials, profile.Credentials)
			profile.Credentials = &merged
		}

		// Merge parent actions into child (child wins on conflict).
		parentActions := parentProfile.Actions
		mergedActions := make(map[string]Action, len(parentActions)+len(profile.Actions))
		for k, v := range parentActions {
			mergedActions[k] = v
		}
		for k, v := range profile.Actions {
			mergedActions[k] = v
		}
		profile.Actions = mergedActions
		cfg.Profiles[name] = profile

		resolved[name] = true
		return nil
	}

	for name := range cfg.Profiles {
		if err := resolve(name); err != nil {
			return err
		}
	}
	return nil
}

// resolveSoundPaths resolves relative sound file paths against the config
// file's directory. Built-in sound names are left unchanged.
func resolveSoundPaths(cfg *Config, configDir string) {
	for pName, profile := range cfg.Profiles {
		for actionName, action := range profile.Actions {
			changed := false
			for i := range action.Steps {
				s := &action.Steps[i]
				if s.Type != "sound" || s.Sound == "" {
					continue
				}
				if _, ok := builtinSounds[s.Sound]; ok {
					continue
				}
				if !filepath.IsAbs(s.Sound) {
					s.Sound = filepath.Join(configDir, s.Sound)
					changed = true
				}
			}
			if changed {
				profile.Actions[actionName] = action
			}
		}
		cfg.Profiles[pName] = profile
	}
}

// MergeCredentials returns global credentials with any non-empty profile
// fields overriding. A nil profile returns global unchanged.
func MergeCredentials(global Credentials, profile *Credentials) Credentials {
	if profile == nil {
		return global
	}
	merged := global
	if profile.DiscordWebhook != "" {
		merged.DiscordWebhook = profile.DiscordWebhook
	}
	if profile.SlackWebhook != "" {
		merged.SlackWebhook = profile.SlackWebhook
	}
	if profile.TelegramToken != "" {
		merged.TelegramToken = profile.TelegramToken
	}
	if profile.TelegramChatID != "" {
		merged.TelegramChatID = profile.TelegramChatID
	}
	return merged
}

// expandEnvCredentials expands $VAR and ${VAR} references in credential
// fields so users can keep secrets in environment variables instead of
// hardcoding them in the JSON config.
func expandEnvCredentials(cfg *Config) {
	expandCreds := func(c *Credentials) {
		c.DiscordWebhook = os.ExpandEnv(c.DiscordWebhook)
		c.SlackWebhook = os.ExpandEnv(c.SlackWebhook)
		c.TelegramToken = os.ExpandEnv(c.TelegramToken)
		c.TelegramChatID = os.ExpandEnv(c.TelegramChatID)
	}
	expandCreds(&cfg.Options.Credentials)
	for pName, profile := range cfg.Profiles {
		if profile.Credentials != nil {
			expandCreds(profile.Credentials)
			cfg.Profiles[pName] = profile
		}
	}
}
