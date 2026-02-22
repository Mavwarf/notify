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

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/cooldown"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/idle"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/silent"
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
	echoFlag := false
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
		case "--echo", "-E":
			echoFlag = true
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
	case "play":
		playCmd(filtered[1:], volume)
	case "history":
		historyCmd(filtered[1:])
	case "silent":
		silentCmd(filtered[1:], configPath, logFlag)
	case "run":
		runWrapped(filtered[1:], configPath, volume, logFlag, echoFlag, cooldownFlag)
	default:
		runAction(filtered, configPath, volume, logFlag, echoFlag, cooldownFlag)
	}
}

func runAction(args []string, configPath string, volume int, logFlag bool, echoFlag bool, cooldownFlag bool) {
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

	if silent.IsSilent() {
		if shouldLog(cfg, logFlag) {
			eventlog.LogSilent(profile, action)
		}
		return
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
	if shouldEcho(cfg, echoFlag) {
		printEcho(filtered)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWrapped(args []string, configPath string, volume int, logFlag bool, echoFlag bool, cooldownFlag bool) {
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

	// Load config and resolve action from exit code.
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	action := resolveExitAction(cfg.Options.ExitCodes, exitCode)

	if silent.IsSilent() {
		if shouldLog(cfg, logFlag) {
			eventlog.LogSilent(profile, action)
		}
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
	if shouldEcho(cfg, echoFlag) {
		printEcho(filtered)
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

// resolveExitAction maps an exit code to an action name using the
// user's exit_codes config. Falls back to "ready" for 0 and "error"
// for any other unmapped code.
func resolveExitAction(codes map[string]string, exitCode int) string {
	if a, ok := codes[strconv.Itoa(exitCode)]; ok {
		return a
	}
	if exitCode == 0 {
		return "ready"
	}
	return "error"
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

// shouldEcho returns true when echo output is enabled via config or flag.
func shouldEcho(cfg config.Config, flag bool) bool {
	return cfg.Options.Echo || flag
}

// printEcho prints a one-line summary of the step types that ran.
func printEcho(steps []config.Step) {
	if len(steps) == 0 {
		fmt.Println("notify: no steps ran")
		return
	}
	seen := map[string]bool{}
	var types []string
	for _, s := range steps {
		if !seen[s.Type] {
			seen[s.Type] = true
			types = append(types, s.Type)
		}
	}
	fmt.Printf("notify: %s\n", strings.Join(types, ", "))
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

func silentCmd(args []string, configPath string, logFlag bool) {
	if len(args) == 0 {
		// Show current status.
		if until, ok := silent.SilentUntil(); ok {
			fmt.Printf("Silent until %s\n", until.Format("15:04:05"))
		} else {
			fmt.Println("Not silent")
		}
		return
	}

	if args[0] == "off" {
		silent.Disable()
		fmt.Println("Silent mode disabled")
		if cfg, err := config.Load(configPath); err == nil && shouldLog(cfg, logFlag) {
			eventlog.LogSilentDisable()
		}
		return
	}

	d, err := time.ParseDuration(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid duration %q (examples: 30s, 5m, 1h, 2h30m)\n", args[0])
		os.Exit(1)
	}
	if d <= 0 {
		fmt.Fprintf(os.Stderr, "Error: duration must be positive\n")
		os.Exit(1)
	}

	silent.Enable(d)
	fmt.Printf("Silent until %s\n", time.Now().Add(d).Format("15:04:05"))
	if cfg, err := config.Load(configPath); err == nil && shouldLog(cfg, logFlag) {
		eventlog.LogSilentEnable(d)
	}
}

func historyCmd(args []string) {
	count := 10
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "Error: count must be a positive integer\n")
			os.Exit(1)
		}
		count = n
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No log file found. Enable logging with --log or \"log\": true in config.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	content := strings.TrimRight(string(data), "\n\r ")
	if content == "" {
		fmt.Println("Log file is empty.")
		return
	}

	entries := strings.Split(content, "\n\n")
	if len(entries) > count {
		entries = entries[len(entries)-count:]
	}
	for i, e := range entries {
		fmt.Print(e)
		fmt.Println()
		if i < len(entries)-1 {
			fmt.Println()
		}
	}
}

func playCmd(args []string, volume int) {
	if len(args) == 0 {
		// List available sounds.
		names := make([]string, 0, len(audio.Sounds))
		for name := range audio.Sounds {
			names = append(names, name)
		}
		sort.Strings(names)

		fmt.Println("Available sounds:")
		for _, name := range names {
			fmt.Printf("  %-14s %s\n", name, audio.Sounds[name].Description)
		}
		return
	}

	name := args[0]
	vol := 1.0
	if volume >= 0 {
		vol = float64(volume) / 100.0
	}
	if err := audio.Play(name, vol); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
		p := cfg.Profiles[pName]
		label := pName
		if p.Extends != "" {
			label = fmt.Sprintf("%s (extends %s)", pName, p.Extends)
		}
		fmt.Printf("%s:\n", label)
		actions := make([]string, 0, len(p.Actions))
		for aName := range p.Actions {
			actions = append(actions, aName)
		}
		sort.Strings(actions)
		for _, aName := range actions {
			act := p.Actions[aName]
			types := make([]string, len(act.Steps))
			for i, s := range act.Steps {
				types[i] = s.Type
			}
			fmt.Printf("  %-20s %s\n", aName, strings.Join(types, ", "))
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

	creds := cfg.Options.Credentials
	fmt.Printf("Discord: %s\n", credStatus(creds.DiscordWebhook != ""))
	fmt.Printf("Slack:   %s\n", credStatus(creds.SlackWebhook != ""))
	fmt.Printf("Telegram:%s\n", credStatus(creds.TelegramToken != "" && creds.TelegramChatID != ""))

	afk := detectAFK(cfg)
	if afk {
		fmt.Printf("AFK:     yes (threshold %ds)\n", cfg.Options.AFKThresholdSeconds)
	} else {
		fmt.Printf("AFK:     no (threshold %ds)\n", cfg.Options.AFKThresholdSeconds)
	}

	if until, ok := silent.SilentUntil(); ok {
		fmt.Printf("Silent:  yes (until %s)\n", until.Format("15:04:05"))
	} else {
		fmt.Printf("Silent:  no\n")
	}

	p, ok := cfg.Profiles[profile]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profile)
		os.Exit(1)
	}

	actionNames := make([]string, 0, len(p.Actions))
	for name := range p.Actions {
		actionNames = append(actionNames, name)
	}
	sort.Strings(actionNames)

	fmt.Printf("\nActions:\n")
	for _, aName := range actionNames {
		act := p.Actions[aName]
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
		if l.Type == s.Type && l.When == s.When && l.Text == s.Text &&
			l.Sound == s.Sound && l.Title == s.Title && l.Message == s.Message &&
			l.URL == s.URL {
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
	case "discord", "discord_voice", "slack", "telegram", "telegram_audio", "telegram_voice":
		parts = append(parts, fmt.Sprintf("text=%q", s.Text))
	case "webhook":
		parts = append(parts, fmt.Sprintf("url=%s", s.URL))
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

func credStatus(ok bool) string {
	if ok {
		return " configured"
	}
	return " not configured"
}

func printVersion() {
	fmt.Printf("notify %s (%s) %s/%s\n", version, buildDate, runtime.GOOS, runtime.GOARCH)
}

func printUsage() {
	fmt.Printf("notify %s - Run notification actions from a config file\n", version)
	fmt.Println(`
Docs: https://github.com/Mavwarf/notify

Usage:
  notify [options] [profile] <action>
  notify run [options] [profile] -- <command...>

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json
  --log, -L              Write invocation to notify.log
  --echo, -E             Print summary of steps that ran
  --cooldown, -C         Enable per-action cooldown (rate limiting)

Commands:
  run                    Wrap a command; map exit code to action (default: 0=ready, else=error)
  play [sound|file.wav]  Preview a built-in sound or WAV file (no args lists built-ins)
  test [profile]         Dry-run: show what would happen without sending
  history [N]            Show last N log entries (default 10)
  silent [duration|off]  Suppress all notifications for a duration (e.g. 1h, 30m)
  list, -l, --list       List all profiles and actions
  version, -V           Show version and build date
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
