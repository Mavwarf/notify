package main

import (
	"encoding/json"
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
	case "config":
		configCmd(filtered[1:], configPath)
	case "send":
		sendCmd(filtered[1:], configPath, volume, logFlag, echoFlag)
	case "silent":
		silentCmd(filtered[1:], configPath, logFlag)
	case "run":
		runWrapped(filtered[1:], configPath, volume, logFlag, echoFlag, cooldownFlag)
	default:
		runAction(filtered, configPath, volume, logFlag, echoFlag, cooldownFlag)
	}
}

func runAction(args []string, configPath string, volume int, logFlag bool, echoFlag bool, cooldownFlag bool) {
	var profile, actionArg string
	switch len(args) {
	case 1:
		profile, actionArg = "default", args[0]
	case 2:
		profile, actionArg = args[0], args[1]
	default:
		fmt.Fprintf(os.Stderr, "Error: expected [profile] <action>\n")
		fmt.Fprintf(os.Stderr, "Run 'notify help' for usage.\n")
		os.Exit(1)
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	volume = resolveVolume(volume, cfg)
	actions := strings.Split(actionArg, ",")
	var failed bool

	for _, action := range actions {
		if silent.IsSilent() {
			if shouldLog(cfg, logFlag) {
				eventlog.LogSilent(profile, action)
			}
			continue
		}

		resolved, act, err := config.Resolve(cfg, profile, action)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			failed = true
			continue
		}

		vars := baseVars(resolved)
		if err := executeAction(cfg, resolved, action, act, volume, logFlag, echoFlag, cooldownFlag, false, vars); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			failed = true
		}
	}

	if failed {
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
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	actionArg := resolveExitAction(cfg.Options.ExitCodes, exitCode)

	volume = resolveVolume(volume, cfg)
	actions := strings.Split(actionArg, ",")

	for _, action := range actions {
		if silent.IsSilent() {
			if shouldLog(cfg, logFlag) {
				eventlog.LogSilent(profile, action)
			}
			continue
		}

		resolved, act, err := config.Resolve(cfg, profile, action)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		vars := baseVars(resolved)
		vars.Command = strings.Join(cmdArgs, " ")
		vars.Duration = formatDuration(elapsed)
		vars.DurationSay = formatDurationSay(elapsed)
		if err := executeAction(cfg, resolved, action, act, volume, logFlag, echoFlag, cooldownFlag, true, vars); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	os.Exit(exitCode)
}

// sendTypes is the set of step types supported by "notify send".
var sendTypes = map[string]bool{
	"say": true, "toast": true,
	"discord": true, "discord_voice": true,
	"slack": true,
	"telegram": true, "telegram_audio": true, "telegram_voice": true,
}

func sendCmd(args []string, configPath string, volume int, logFlag bool, echoFlag bool) {
	// Parse optional --title flag (for toast).
	var title string
	rest := make([]string, len(args))
	copy(rest, args)
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--title" && i+1 < len(rest) {
			title = rest[i+1]
			rest = append(rest[:i], rest[i+2:]...)
			break
		}
	}

	if len(rest) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: notify send [--title <title>] <type> <message>\n")
		os.Exit(1)
	}

	stepType := rest[0]
	message := rest[1]

	if !sendTypes[stepType] {
		fmt.Fprintf(os.Stderr, "Error: unsupported send type %q\n", stepType)
		fmt.Fprintf(os.Stderr, "Supported: say, toast, discord, discord_voice, slack, telegram, telegram_audio, telegram_voice\n")
		os.Exit(1)
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	volume = resolveVolume(volume, cfg)

	// Build a single step from the positional args.
	step := config.Step{Type: stepType}
	if stepType == "toast" {
		step.Message = message
		if title != "" {
			step.Title = title
		}
	} else {
		step.Text = message
	}

	vars := baseVars("send")
	steps := []config.Step{step}
	if err := runner.Execute(steps, volume, cfg.Options.Credentials, vars); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if shouldLog(cfg, logFlag) {
		eventlog.Log("send:"+stepType, steps, false, vars)
	}
	if shouldEcho(cfg, echoFlag) {
		printEcho(steps)
	}
}

// loadAndValidate loads and validates the config file, returning an error
// on any problem (instead of calling os.Exit directly).
func loadAndValidate(configPath string) (config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, err
	}
	if err := config.Validate(cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

// executeAction runs the common tail of runAction/runWrapped: cooldown check,
// step filtering, execution, cooldown recording, logging, and echo output.
func executeAction(cfg config.Config, profile, action string, act *config.Action,
	volume int, logFlag, echoFlag, cooldownFlag, run bool, vars tmpl.Vars) error {

	cdEnabled, cdSec := resolveCooldown(act, cfg, cooldownFlag)
	if cdEnabled && cdSec > 0 && cooldown.Check(profile, action, cdSec) {
		if shouldLog(cfg, logFlag) {
			eventlog.LogCooldown(profile, action, cdSec)
		}
		return nil
	}

	afk := detectAFK(cfg)

	creds := config.MergeCredentials(cfg.Options.Credentials, cfg.Profiles[profile].Credentials)

	filtered := runner.FilterSteps(act.Steps, afk, run)
	err := runner.Execute(filtered, volume, creds, vars)
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
	return err
}

// baseVars returns a Vars with profile, time, date, and hostname pre-filled.
func baseVars(profile string) tmpl.Vars {
	host, _ := os.Hostname()
	now := time.Now()
	return tmpl.Vars{
		Profile:  profile,
		Time:     now.Format("15:04"),
		TimeSay:  now.Format("3:04 PM"),
		Date:     now.Format("2006-01-02"),
		DateSay:  now.Format("January 2, 2006"),
		Hostname: host,
	}
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
		parts = append(parts, pluralize(hours, "hour", "hours"))
	}
	if minutes > 0 {
		parts = append(parts, pluralize(minutes, "minute", "minutes"))
	}
	if seconds > 0 {
		parts = append(parts, pluralize(seconds, "second", "seconds"))
	}

	if len(parts) == 0 {
		return "0 seconds"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// pluralize returns "1 singular" or "N plural".
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
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
	if len(args) > 0 {
		switch args[0] {
		case "summary":
			historySummary(args[1:])
			return
		case "clear":
			historyClear()
			return
		case "export":
			historyExport(args[1:])
			return
		}
	}

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

func historySummary(args []string) {
	days := 7
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "Error: days must be a positive integer\n")
			os.Exit(1)
		}
		days = n
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

	entries := eventlog.ParseEntries(string(data))
	groups := eventlog.SummarizeByDay(entries, days)

	if len(groups) == 0 {
		fmt.Println("No activity in the last", days, "days.")
		return
	}

	totalExec := 0
	totalSkip := 0
	for i, dg := range groups {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s  (%s)\n", dg.Date.Format("2006-01-02"), dg.Date.Format("Monday"))
		for _, s := range dg.Summaries {
			label := s.Profile + "/" + s.Action
			if s.Skipped > 0 {
				fmt.Printf("  %-24s %d   (%d skipped)\n", label, s.Executions, s.Skipped)
			} else {
				fmt.Printf("  %-24s %d\n", label, s.Executions)
			}
			totalExec += s.Executions
			totalSkip += s.Skipped
		}
	}

	fmt.Println()
	if totalSkip > 0 {
		fmt.Printf("Total: %d executions, %d skipped\n", totalExec, totalSkip)
	} else {
		fmt.Printf("Total: %d executions\n", totalExec)
	}
}

func historyClear() {
	path := eventlog.LogPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Log file cleared.")
}

func configCmd(args []string, configPath string) {
	if len(args) == 0 || args[0] == "validate" {
		configValidate(configPath)
		return
	}
	fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
	os.Exit(1)
}

func configValidate(configPath string) {
	p, err := config.FindPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if _, err := loadAndValidate(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Config OK: %s\n", p)
}

func historyExport(args []string) {
	days := 0
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "Error: days must be a positive integer\n")
			os.Exit(1)
		}
		days = n
	}

	path := eventlog.LogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("[]")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	entries := eventlog.ParseEntries(string(data))

	if days > 0 {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff := today.AddDate(0, 0, -(days - 1))
		var filtered []eventlog.Entry
		for _, e := range entries {
			if !e.Time.In(now.Location()).Before(cutoff) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	type exportEntry struct {
		Time    string `json:"time"`
		Profile string `json:"profile"`
		Action  string `json:"action"`
		Kind    string `json:"kind"`
	}
	out := make([]exportEntry, len(entries))
	for i, e := range entries {
		out[i] = exportEntry{
			Time:    e.Time.Format(time.RFC3339),
			Profile: e.Profile,
			Action:  e.Action,
			Kind:    eventlog.KindString(e.Kind),
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
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
	cfg, err := loadAndValidate(configPath)
	if err != nil {
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
		var annotations []string
		if p.Extends != "" {
			annotations = append(annotations, fmt.Sprintf("extends %s", p.Extends))
		}
		if len(p.Aliases) > 0 {
			annotations = append(annotations, fmt.Sprintf("aliases: %s", strings.Join(p.Aliases, ", ")))
		}
		if len(annotations) > 0 {
			label = fmt.Sprintf("%s (%s)", pName, strings.Join(annotations, ", "))
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

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config:  OK\n")
	fmt.Printf("Profile: %s\n", profile)
	fmt.Printf("Volume:  %d\n", cfg.Options.DefaultVolume)

	creds := config.MergeCredentials(cfg.Options.Credentials, cfg.Profiles[profile].Credentials)
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
		wouldRun := runner.FilteredIndices(act.Steps, afk, false)
		fmt.Printf("\n  %s (%d/%d steps would run):\n", aName, len(wouldRun), len(act.Steps))
		for i, s := range act.Steps {
			marker := "  SKIP "
			if wouldRun[i] {
				marker = "  RUN  "
			}
			detail := stepSummary(s)
			fmt.Printf("    %s[%d] %-10s %s\n", marker, i+1, s.Type, detail)
		}
	}
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
  notify [options] [profile] <action[,action2,...]>
  notify run [options] [profile] -- <command...>
  notify send [--title <title>] <type> <message>

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json
  --log, -L              Write invocation to notify.log
  --echo, -E             Print summary of steps that ran
  --cooldown, -C         Enable per-action cooldown (rate limiting)

Commands:
  send <type> <message>  Send a one-off notification (e.g. send telegram "hello")
                         Supported: say, toast, discord, discord_voice, slack,
                         telegram, telegram_audio, telegram_voice
                         --title <title>  Set toast title (toast only)
  run                    Wrap a command; map exit code to action (default: 0=ready, else=error)
  play [sound|file.wav]  Preview a built-in sound or WAV file (no args lists built-ins)
  test [profile]         Dry-run: show what would happen without sending
  config validate        Check config file for errors
  history [N]            Show last N log entries (default 10)
  history summary [days] Show action counts per day (default 7 days)
  history export [days]  Export log entries as JSON (default: all)
  history clear          Delete the log file
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
  notify boss done,attention       Run "done" then "attention" from boss
  notify -v 50 ready               Run at 50% volume
  notify run -- make build         Wrap a command (default profile)
  notify run boss -- cargo test    Wrap with a specific profile

Created by Thomas Häuser
https://mavwarf.netlify.app/
https://github.com/Mavwarf/notify`)
}
