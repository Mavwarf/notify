package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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

// lockedWriter is a concurrency-safe bytes.Buffer used to capture
// stdout and stderr from a wrapped command simultaneously.
type lockedWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// matchPair associates a substring pattern with an action name.
type matchPair struct {
	pattern string
	action  string
}

func main() {
	args := os.Args[1:]
	volume := -1
	configPath := ""
	logFlag := false
	echoFlag := false
	cooldownFlag := false
	heartbeatSec := 0
	var matches []matchPair

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
		case "--match", "-M":
			if i+2 < len(args) {
				matches = append(matches, matchPair{pattern: args[i+1], action: args[i+2]})
				i += 2
			} else {
				fmt.Fprintf(os.Stderr, "Error: --match requires <pattern> <action>\n")
				os.Exit(1)
			}
		case "--log", "-L":
			logFlag = true
		case "--echo", "-E":
			echoFlag = true
		case "--cooldown", "-C":
			cooldownFlag = true
		case "--heartbeat", "-H":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err != nil || d <= 0 {
					fmt.Fprintf(os.Stderr, "Error: --heartbeat requires a positive duration (e.g. 5m, 2m30s)\n")
					os.Exit(1)
				}
				heartbeatSec = int(d.Seconds())
				if heartbeatSec == 0 {
					heartbeatSec = 1 // sub-second rounds up to 1s
				}
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --heartbeat requires a duration (e.g. 5m, 2m30s)\n")
				os.Exit(1)
			}
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
		runWrapped(filtered[1:], configPath, volume, logFlag, echoFlag, cooldownFlag, matches, heartbeatSec)
	case "pipe":
		runPipe(filtered[1:], configPath, volume, logFlag, echoFlag, cooldownFlag, matches)
	default:
		runAction(filtered, configPath, volume, logFlag, echoFlag, cooldownFlag)
	}
}

// cwd returns the current working directory, or "" if it cannot be determined.
func cwd() string {
	dir, _ := os.Getwd()
	return dir
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

	if len(args) == 1 {
		profile = config.MatchProfile(cfg, cwd())
	}

	if err := dispatchActions(cfg, profile, actionArg, volume, logFlag, echoFlag, cooldownFlag, false, nil); err != nil {
		os.Exit(1)
	}
}

func runWrapped(args []string, configPath string, volume int, logFlag bool, echoFlag bool, cooldownFlag bool, matches []matchPair, heartbeatFlag int) {
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

	// Load config early so we can auto-select profile before running command.
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if sepIdx == 0 {
		profile = config.MatchProfile(cfg, cwd())
	}

	// Determine whether output capture is needed.
	captureOutput := len(matches) > 0 || cfg.Options.OutputLines > 0

	// Execute the wrapped command.
	start := time.Now()
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin

	var captured *lockedWriter
	if captureOutput {
		captured = &lockedWriter{}
		cmd.Stdout = io.MultiWriter(os.Stdout, captured)
		cmd.Stderr = io.MultiWriter(os.Stderr, captured)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Start heartbeat goroutine if enabled.
	hbSec := resolveHeartbeat(cfg, heartbeatFlag)
	done := make(chan struct{})
	if hbSec > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(hbSec) * time.Second)
			defer ticker.Stop()
			cmdStr := strings.Join(cmdArgs, " ")
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					elapsed := time.Since(start)
					dispatchActions(cfg, profile, "heartbeat", volume, logFlag, echoFlag, cooldownFlag, true,
						func(v *tmpl.Vars) {
							v.Command = cmdStr
							v.Duration = formatDuration(elapsed)
							v.DurationSay = formatDurationSay(elapsed)
						})
				}
			}
		}()
	}

	cmdErr := cmd.Run()
	close(done)
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

	// Resolve action: match patterns → exit codes → default.
	var fullOutput string
	if captured != nil {
		fullOutput = captured.String()
	}

	actionArg := resolveMatchAction(matches, fullOutput)
	if actionArg == "" {
		actionArg = resolveExitAction(cfg.Options.ExitCodes, exitCode)
	}

	// Extract last N lines for {output} template variable.
	var outputSnippet string
	if cfg.Options.OutputLines > 0 && fullOutput != "" {
		outputSnippet = lastNLines(fullOutput, cfg.Options.OutputLines)
	}

	// Error deliberately ignored: the wrapped command's exit code takes
	// priority so the caller can distinguish command failure from notify failure.
	dispatchActions(cfg, profile, actionArg, volume, logFlag, echoFlag, cooldownFlag, true,
		func(v *tmpl.Vars) {
			v.Command = strings.Join(cmdArgs, " ")
			v.Duration = formatDuration(elapsed)
			v.DurationSay = formatDurationSay(elapsed)
			v.Output = outputSnippet
		})

	os.Exit(exitCode)
}

func runPipe(args []string, configPath string, volume int, logFlag bool, echoFlag bool, cooldownFlag bool, matches []matchPair) {
	// Parse optional profile from args[0], default "default".
	profile := "default"
	if len(args) > 0 {
		profile = args[0]
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		profile = config.MatchProfile(cfg, cwd())
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		var actionArg string
		if len(matches) > 0 {
			actionArg = resolveMatchAction(matches, line)
			if actionArg == "" {
				continue // no pattern matched, skip silently
			}
		} else {
			actionArg = "ready"
		}

		dispatchActions(cfg, profile, actionArg, volume, logFlag, echoFlag, cooldownFlag, false,
			func(v *tmpl.Vars) {
				v.Output = line
			})
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}
}

// dispatchActions is the shared action loop for runAction and runWrapped.
// It iterates over comma-separated actions, checks silent mode, resolves
// each action, applies optional extraVars, and calls executeAction.
// Returns a non-nil error if any action failed.
func dispatchActions(cfg config.Config, profile, actionArg string,
	volume int, logFlag, echoFlag, cooldownFlag, runMode bool,
	extraVars func(*tmpl.Vars)) error {

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
		if extraVars != nil {
			extraVars(&vars)
		}
		if err := executeAction(cfg, resolved, action, act, volume, logFlag, echoFlag, cooldownFlag, runMode, vars); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("one or more actions failed")
	}
	return nil
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

// resolveHeartbeat returns the effective heartbeat interval in seconds.
// CLI flag wins if > 0, otherwise config value is used. 0 = disabled.
func resolveHeartbeat(cfg config.Config, flagSec int) int {
	if flagSec > 0 {
		return flagSec
	}
	return cfg.Options.HeartbeatSeconds
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

// resolveMatchAction scans output for the first matching pattern and
// returns the associated action name. Returns "" if no pattern matches.
func resolveMatchAction(matches []matchPair, output string) string {
	for _, m := range matches {
		if strings.Contains(output, m.pattern) {
			return m.action
		}
	}
	return ""
}

// lastNLines returns the last n lines of s, trimming a trailing newline.
func lastNLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
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
  notify pipe [options] [profile] [--match <pattern> <action>...]
  notify send [--title <title>] <type> <message>

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json
  --match, -M <pat> <action>  Select action by output pattern (repeatable, run/pipe mode)
  --log, -L              Write invocation to notify.log
  --echo, -E             Print summary of steps that ran
  --cooldown, -C         Enable per-action cooldown (rate limiting)
  --heartbeat, -H <dur>  Periodic notification during "run" (e.g. 5m, 2m30s)

Commands:
  send <type> <message>  Send a one-off notification (e.g. send telegram "hello")
                         Supported: say, toast, discord, discord_voice, slack,
                         telegram, telegram_audio, telegram_voice
                         --title <title>  Set toast title (toast only)
  pipe [profile]         Read stdin line-by-line, trigger action on pattern match
                         Without --match, every line triggers "ready"
                         {output} = the matched line
  run                    Wrap a command; map exit code or output pattern to action
                         --heartbeat/-H fires the "heartbeat" action periodically
  play [sound|file.wav]  Preview a built-in sound or WAV file (no args lists built-ins)
  test [profile]         Dry-run: show what would happen without sending
  config validate        Check config file for errors
  history [N]            Show last N log entries (default 10)
  history summary [days|all] Show action counts per day (default 7 days)
  history watch          Live today's summary (refreshes every 2s, press x to exit)
  history export [days]  Export log entries as JSON (default: all)
  history clean [days]   Remove old entries, keep last N days (no arg = clear all)
  history clear          Delete the log file
  silent [duration|off]  Suppress all notifications for a duration (e.g. 1h, 30m)
  list, -l, --list       List all profiles and actions
  version, -V           Show version and build date
  help, -h, --help       Show this help message

Config resolution:
  1. --config <path>              (explicit)
  2. notify-config.json next to binary   (portable)
  3. ~/.config/notify/notify-config.json (user default)

Profile auto-selection:
  When profile is omitted, match rules select a profile by working
  directory ("dir") or environment variable ("env"). First alphabetical
  match wins. Falls back to "default" if none match.

Template variables:
  {profile}, {Profile}, {time}, {Time}, {date}, {Date}, {hostname}
  Run mode: {command}, {duration}, {Duration}, {output}
  Pipe mode: {output} (the matched line)

  In run mode, {output} contains the last N lines of command output when
  "output_lines" is set in config. In pipe mode, {output} is the matched line.

Examples:
  notify ready                     Run "ready" from the default profile
  notify boss ready                Run "ready" from the boss profile
  notify boss done,attention       Run "done" then "attention" from boss
  notify -v 50 ready               Run at 50% volume
  notify run -- make build         Wrap a command (default profile)
  notify run boss -- cargo test    Wrap with a specific profile
  notify run --heartbeat 5m -- make build
                                   Heartbeat every 5 minutes during build
  notify run -H 2m boss -- cargo test
                                   Heartbeat every 2 minutes for boss profile
  notify run -M FAIL error -M passed ready -- pytest
                                   Select action by output pattern
  tail -f build.log | notify pipe boss -M SUCCESS done -M FAIL error
                                   Pipe mode: match patterns in stream
  deploy-events | notify pipe ops  Every line triggers "ready"

Created by Thomas Häuser
https://mavwarf.netlify.app/
https://github.com/Mavwarf/notify`)
}
