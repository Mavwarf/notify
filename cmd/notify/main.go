package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/cooldown"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/idle"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/tmpl"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	args := os.Args[1:]
	volume := -1
	configPath := ""
	logFlag := false
	cooldownFlag := false

	// Parse flags
	filtered := args[:0]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--volume", "-v":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err != nil || v < 0 || v > 100 {
					fmt.Fprintf(os.Stderr, "Error: volume must be a number between 0 and 100\n")
					os.Exit(1)
				}
				volume = v
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --volume requires a value (0-100)\n")
				os.Exit(1)
			}
		case "--config", "-c":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --config requires a file path\n")
				os.Exit(1)
			}
		case "--log", "-L":
			logFlag = true
		case "--cooldown", "-C":
			cooldownFlag = true
		default:
			filtered = append(filtered, args[i])
		}
	}

	if len(filtered) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch filtered[0] {
	case "help", "-h", "--help":
		printUsage()
	case "version", "-V", "--version":
		printVersion()
	case "list", "-l", "--list":
		listProfiles(configPath)
	case "test":
		dryRun(filtered[1:], configPath)
	case "run":
		runWrapped(filtered[1:], configPath, volume, logFlag, cooldownFlag)
	default:
		runAction(filtered, configPath, volume, logFlag, cooldownFlag)
	}
}

func runAction(args []string, configPath string, volume int, logFlag bool, cooldownFlag bool) {
	var profile, action string
	switch len(args) {
	case 1:
		profile, action = "default", args[0]
	case 2:
		profile, action = args[0], args[1]
	default:
		fmt.Fprintf(os.Stderr, "Error: expected [profile] <action>\n")
		fmt.Fprintf(os.Stderr, "Run 'notify help' for usage.\n")
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	volume = resolveVolume(volume, cfg)

	act, err := config.Resolve(cfg, profile, action)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cdEnabled, cdSec := resolveCooldown(act, cfg, cooldownFlag)
	if cdEnabled && cdSec > 0 && cooldown.Check(profile, action, cdSec) {
		if shouldLog(cfg, logFlag) {
			eventlog.LogCooldown(profile, action, cdSec)
		}
		return
	}

	afk := detectAFK(cfg)

	vars := tmpl.Vars{Profile: profile}
	filtered := runner.FilterSteps(act.Steps, afk, false)
	err = runner.Execute(act, volume, cfg.Options.Credentials, vars, afk, false)
	if cdEnabled && cdSec > 0 {
		cooldown.Record(profile, action)
		if shouldLog(cfg, logFlag) {
			eventlog.LogCooldownRecord(profile, action, cdSec)
		}
	}
	if shouldLog(cfg, logFlag) {
		eventlog.Log(action, filtered, afk, vars)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWrapped(args []string, configPath string, volume int, logFlag bool, cooldownFlag bool) {
	// Find "--" separator.
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}

	if sepIdx < 0 {
		fmt.Fprintf(os.Stderr, "Error: 'notify run' requires '--' before the command\n")
		fmt.Fprintf(os.Stderr, "Usage: notify run [profile] -- <command...>\n")
		os.Exit(1)
	}

	cmdArgs := args[sepIdx+1:]
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified after '--'\n")
		fmt.Fprintf(os.Stderr, "Usage: notify run [profile] -- <command...>\n")
		os.Exit(1)
	}

	// Everything before "--" is the optional profile.
	profile := "default"
	if sepIdx > 0 {
		profile = args[sepIdx-1]
	}

	// Execute the wrapped command.
	start := time.Now()
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmdErr := cmd.Run()
	elapsed := time.Since(start)

	// Determine exit code and action.
	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
			os.Exit(1)
		}
	}

	action := "ready"
	if exitCode != 0 {
		action = "error"
	}

	// Load config and run notification.
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	volume = resolveVolume(volume, cfg)

	act, err := config.Resolve(cfg, profile, action)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	cdEnabled, cdSec := resolveCooldown(act, cfg, cooldownFlag)
	if cdEnabled && cdSec > 0 && cooldown.Check(profile, action, cdSec) {
		if shouldLog(cfg, logFlag) {
			eventlog.LogCooldown(profile, action, cdSec)
		}
		os.Exit(exitCode)
	}

	afk := detectAFK(cfg)

	vars := tmpl.Vars{
		Profile:     profile,
		Command:     strings.Join(cmdArgs, " "),
		Duration:    formatDuration(elapsed),
		DurationSay: formatDurationSay(elapsed),
	}
	filtered := runner.FilterSteps(act.Steps, afk, true)
	err = runner.Execute(act, volume, cfg.Options.Credentials, vars, afk, true)
	if cdEnabled && cdSec > 0 {
		cooldown.Record(profile, action)
		if shouldLog(cfg, logFlag) {
			eventlog.LogCooldownRecord(profile, action, cdSec)
		}
	}
	if shouldLog(cfg, logFlag) {
		eventlog.Log(action, filtered, afk, vars)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	os.Exit(exitCode)
}

// resolveVolume returns the CLI volume if set, otherwise the config default.
func resolveVolume(volume int, cfg config.Config) int {
	if volume < 0 {
		return cfg.Options.DefaultVolume
	}
	return volume
}

// resolveCooldown returns whether cooldown is enabled and the effective
// cooldown duration in seconds (per-action overrides global).
func resolveCooldown(act *config.Action, cfg config.Config, flag bool) (bool, int) {
	enabled := cfg.Options.Cooldown || flag
	sec := act.CooldownSeconds
	if sec == 0 {
		sec = cfg.Options.CooldownSeconds
	}
	return enabled, sec
}

// idleFunc is the function used to get idle time. Replaced in tests.
var idleFunc = idle.IdleSeconds

// detectAFK returns true when the user has been idle longer than the
// configured threshold. Fails open (returns false on error).
func detectAFK(cfg config.Config) bool {
	idleSec, err := idleFunc()
	if err != nil {
		return false
	}
	return idleSec >= float64(cfg.Options.AFKThresholdSeconds)
}

// shouldLog returns true when event logging is enabled via config or flag.
func shouldLog(cfg config.Config, flag bool) bool {
	return cfg.Options.Log || flag
}

// formatDuration returns a compact duration string (e.g. "3s", "2m15s").
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	d = d.Round(time.Second)
	return d.String()
}

// formatDurationSay returns a spoken-friendly duration string
// (e.g. "2 minutes and 15 seconds").
func formatDurationSay(d time.Duration) string {
	if d < time.Second {
		return "less than a second"
	}

	total := int(d.Round(time.Second).Seconds())
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	var parts []string
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}

	if len(parts) == 0 {
		return "0 seconds"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func listProfiles(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	profiles := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)

	for _, pName := range profiles {
		fmt.Printf("%s:\n", pName)
		actions := make([]string, 0, len(cfg.Profiles[pName]))
		for aName := range cfg.Profiles[pName] {
			actions = append(actions, aName)
		}
		sort.Strings(actions)
		for _, aName := range actions {
			act := cfg.Profiles[pName][aName]
			fmt.Printf("  %-20s (%d steps)\n", aName, len(act.Steps))
		}
	}
}

func dryRun(args []string, configPath string) {
	profile := "default"
	if len(args) > 0 {
		profile = args[0]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config:  OK\n")
	fmt.Printf("Profile: %s\n", profile)
	fmt.Printf("Volume:  %d\n", cfg.Options.DefaultVolume)

	afk := detectAFK(cfg)
	if afk {
		fmt.Printf("AFK:     yes (threshold %ds)\n", cfg.Options.AFKThresholdSeconds)
	} else {
		fmt.Printf("AFK:     no (threshold %ds)\n", cfg.Options.AFKThresholdSeconds)
	}

	p, ok := cfg.Profiles[profile]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profile)
		os.Exit(1)
	}

	actionNames := make([]string, 0, len(p))
	for name := range p {
		actionNames = append(actionNames, name)
	}
	sort.Strings(actionNames)

	fmt.Printf("\nActions:\n")
	for _, aName := range actionNames {
		act := p[aName]
		filtered := runner.FilterSteps(act.Steps, afk, false)
		fmt.Printf("\n  %s (%d/%d steps would run):\n", aName, len(filtered), len(act.Steps))
		for i, s := range act.Steps {
			marker := "  "
			if !stepInList(s, filtered) {
				marker = "  SKIP "
			} else {
				marker = "  RUN  "
			}
			detail := stepSummary(s)
			fmt.Printf("    %s[%d] %-10s %s\n", marker, i+1, s.Type, detail)
		}
	}
}

func stepInList(s config.Step, list []config.Step) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

func stepSummary(s config.Step) string {
	parts := []string{}
	switch s.Type {
	case "sound":
		parts = append(parts, fmt.Sprintf("sound=%s", s.Sound))
	case "say":
		parts = append(parts, fmt.Sprintf("text=%q", s.Text))
	case "toast":
		if s.Title != "" {
			parts = append(parts, fmt.Sprintf("title=%q", s.Title))
		}
		parts = append(parts, fmt.Sprintf("message=%q", s.Message))
	case "discord", "discord_voice", "telegram", "telegram_audio":
		parts = append(parts, fmt.Sprintf("text=%q", s.Text))
	}
	if s.When != "" {
		parts = append(parts, fmt.Sprintf("when=%s", s.When))
	}
	if s.Volume != nil {
		parts = append(parts, fmt.Sprintf("volume=%d", *s.Volume))
	}
	return strings.Join(parts, "  ")
}

func printVersion() {
	fmt.Printf("notify %s (%s) %s/%s\n", version, buildDate, runtime.GOOS, runtime.GOARCH)
}

func printUsage() {
	fmt.Printf("notify %s - Run notification actions from a config file\n", version)
	fmt.Println(`
Usage:
  notify [options] [profile] <action>
  notify run [options] [profile] -- <command...>

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json
  --log, -L              Write invocation to notify.log
  --cooldown, -C         Enable per-action cooldown (rate limiting)

Commands:
  run                    Wrap a command; notify ready on success, error on failure
  test [profile]         Dry-run: show what would happen without sending
  list, -l, --list       List all profiles and actions
  version, -V             Show version and build date
  help, -h, --help       Show this help message

Config resolution:
  1. --config <path>              (explicit)
  2. notify-config.json next to binary   (portable)
  3. ~/.config/notify/notify-config.json (user default)

Examples:
  notify ready                     Run "ready" from the default profile
  notify boss ready                Run "ready" from the boss profile
  notify -v 50 ready               Run at 50% volume
  notify run -- make build         Wrap a command (default profile)
  notify run boss -- cargo test    Wrap with a specific profile

Created by Thomas Häuser
https://mavwarf.netlify.app/
https://github.com/Mavwarf/notify`)
}
