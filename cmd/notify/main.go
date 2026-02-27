package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"net/url"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/cooldown"
	"github.com/Mavwarf/notify/internal/desktop"
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

// runOpts groups the common flags threaded through runAction, runWrapped,
// runPipe, dispatchActions, and executeAction. Adding a new opt-in flag
// only requires adding a field here — no function signature changes.
type runOpts struct {
	Volume   int
	Log      bool
	Echo     bool
	Cooldown bool
	RunMode  bool
	Elapsed  time.Duration
	Delay    time.Duration
	AtTime   string
}

// readLog reads the event log via the default Store. Returns the content
// and true on success. If the log is empty, returns ("", false) so the
// caller can print a context-appropriate message. Fatals on read errors.
func readLog() (string, bool) {
	data, err := eventlog.ReadContent()
	if err != nil {
		fatal("%v", err)
	}
	if data == "" {
		return "", false
	}
	return data, true
}

// fatal prints an error message to stderr and exits with code 1.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	volume := -1
	configPath := ""
	logFlag := false
	echoFlag := false
	cooldownFlag := false
	heartbeatSec := 0
	port := 8080
	openFlag := false
	protocolURI := ""
	var delayDur time.Duration
	var atTime string
	var matches []matchPair

	// Parse flags
	filtered := args[:0]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--volume", "-v":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err != nil || v < 0 || v > 100 {
					fatal("volume must be a number between 0 and 100")
				}
				volume = v
				i++
			} else {
				fatal("--volume requires a value (0-100)")
			}
		case "--config", "-c":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			} else {
				fatal("--config requires a file path")
			}
		case "--match", "-M":
			if i+2 < len(args) {
				matches = append(matches, matchPair{pattern: args[i+1], action: args[i+2]})
				i += 2
			} else {
				fatal("--match requires <pattern> <action>")
			}
		case "--log", "-L":
			logFlag = true
		case "--echo", "-E":
			echoFlag = true
		case "--cooldown", "-C":
			cooldownFlag = true
		case "--open", "-O":
			openFlag = true
		case "--protocol":
			if i+1 < len(args) {
				protocolURI = args[i+1]
				i++
			} else {
				fatal("--protocol requires a URI")
			}
		case "--port", "-p":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err != nil || v < 1 || v > 65535 {
					fatal("port must be a number between 1 and 65535")
				}
				port = v
				i++
			} else {
				fatal("--port requires a value (1-65535)")
			}
		case "--heartbeat", "-H":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err != nil || d <= 0 {
					fatal("--heartbeat requires a positive duration (e.g. 5m, 2m30s)")
				}
				heartbeatSec = int(d.Seconds())
				if heartbeatSec == 0 {
					heartbeatSec = 1 // sub-second rounds up to 1s
				}
				i++
			} else {
				fatal("--heartbeat requires a duration (e.g. 5m, 2m30s)")
			}
		case "--delay", "-D":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err != nil || d <= 0 {
					fatal("--delay requires a positive duration (e.g. 5s, 10m, 1h)")
				}
				delayDur = d
				i++
			} else {
				fatal("--delay requires a duration (e.g. 5s, 10m, 1h)")
			}
		case "--at", "-A":
			if i+1 < len(args) {
				atTime = args[i+1]
				i++
			} else {
				fatal("--at requires a time (e.g. 14:30, 2:30PM)")
			}
		default:
			filtered = append(filtered, args[i])
		}
	}

	// Initialize storage backend from config.
	storage := ""
	if cfg, err := config.Load(configPath); err == nil {
		storage = cfg.Options.Storage
		eventlog.SetRetention(cfg.Options.RetentionDays)
	}
	eventlog.OpenDefault(storage)
	defer eventlog.Close()

	// Handle --protocol activation (from toast click).
	if protocolURI != "" {
		handleProtocolURI(protocolURI)
		return
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
	case "voice":
		voiceCmd(filtered[1:], configPath)
	case "history":
		historyCmd(filtered[1:])
	case "dashboard":
		dashboardCmd(configPath, port, openFlag)
	case "startup":
		startupCmd(configPath, port, openFlag)
	case "init":
		initCmd(filtered[1:], configPath)
	case "config":
		configCmd(filtered[1:], configPath)
	case "protocol":
		protocolCmd(filtered[1:])
	case "send":
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag}
		sendCmd(filtered[1:], configPath, opts)
	case "silent":
		silentCmd(filtered[1:], configPath, logFlag)
	case "run":
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag, Cooldown: cooldownFlag}
		runWrapped(filtered[1:], configPath, opts, matches, heartbeatSec)
	case "watch":
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag, Cooldown: cooldownFlag}
		watchCmd(filtered[1:], configPath, opts)
	case "shell-hook":
		shellHookCmd(filtered[1:], configPath)
	case "_hook":
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag, Cooldown: cooldownFlag}
		hookCmd(filtered[1:], configPath, opts)
	case "pipe":
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag, Cooldown: cooldownFlag}
		runPipe(filtered[1:], configPath, opts, matches)
	default:
		opts := runOpts{Volume: volume, Log: logFlag, Echo: echoFlag, Cooldown: cooldownFlag, Delay: delayDur, AtTime: atTime}
		runAction(filtered, configPath, opts)
	}
}

// cwd returns the current working directory, or "" if it cannot be determined.
func cwd() string {
	dir, _ := os.Getwd()
	return dir
}

func runAction(args []string, configPath string, opts runOpts) {
	var profile, actionArg string
	explicit := len(args) == 2
	switch len(args) {
	case 1:
		profile, actionArg = "default", args[0]
	case 2:
		profile, actionArg = args[0], args[1]
	default:
		fatal("expected [profile] <action>\nRun 'notify help' for usage.")
	}

	// Read piped JSON from stdin (e.g. from Claude Code hooks).
	stdinData := stdinReader()

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}

	profile = resolveProfile(cfg, profile, explicit)

	// Handle scheduled reminders (--delay / --at).
	if opts.Delay > 0 || opts.AtTime != "" {
		sleepForReminder(opts.Delay, opts.AtTime, actionArg)
	}

	if err := dispatchActions(cfg, profile, actionArg, opts, stdinVars(stdinData)); err != nil {
		os.Exit(1)
	}
}

func runWrapped(args []string, configPath string, opts runOpts, matches []matchPair, heartbeatFlag int) {
	opts.RunMode = true

	// Find "--" separator.
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}

	if sepIdx < 0 {
		fatal("'notify run' requires '--' before the command\nUsage: notify run [profile] -- <command...>")
	}

	cmdArgs := args[sepIdx+1:]
	if len(cmdArgs) == 0 {
		fatal("no command specified after '--'\nUsage: notify run [profile] -- <command...>")
	}

	// Everything before "--" is the optional profile.
	profile := "default"
	explicit := sepIdx > 0
	if explicit {
		profile = args[sepIdx-1]
	}

	// Load config early so we can auto-select profile before running command.
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}

	profile = resolveProfile(cfg, profile, explicit)

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
					hbOpts := opts
					hbOpts.Elapsed = elapsed
					dispatchActions(cfg, profile, "heartbeat", hbOpts,
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
			fatal("%v", cmdErr)
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
	opts.Elapsed = elapsed
	dispatchActions(cfg, profile, actionArg, opts,
		func(v *tmpl.Vars) {
			v.Command = strings.Join(cmdArgs, " ")
			v.Duration = formatDuration(elapsed)
			v.DurationSay = formatDurationSay(elapsed)
			v.Output = outputSnippet
		})

	os.Exit(exitCode)
}

func runPipe(args []string, configPath string, opts runOpts, matches []matchPair) {
	// Parse optional profile from args[0], default "default".
	profile := "default"
	explicit := len(args) > 0
	if explicit {
		profile = args[0]
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}

	profile = resolveProfile(cfg, profile, explicit)

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

		dispatchActions(cfg, profile, actionArg, opts,
			func(v *tmpl.Vars) {
				v.Output = line
			})
	}

	if err := scanner.Err(); err != nil {
		fatal("reading stdin: %v", err)
	}
}

// dispatchActions is the shared action loop for runAction and runWrapped.
// It iterates over comma-separated actions, checks silent mode, resolves
// each action, applies optional extraVars, and calls executeAction.
// Returns a non-nil error if any action failed.
func dispatchActions(cfg config.Config, profile, actionArg string,
	opts runOpts, extraVars func(*tmpl.Vars)) error {

	opts.Volume = resolveVolume(opts.Volume, cfg)
	actions := strings.Split(actionArg, ",")
	var failed bool

	for _, action := range actions {
		if silent.IsSilent() {
			if shouldLog(cfg, opts.Log) {
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
		if err := executeAction(cfg, resolved, action, act, opts, vars); err != nil {
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
	if cfg.Builtin {
		fmt.Fprintf(os.Stderr, "notify: using built-in defaults (create notify-config.json to customize)\n")
	}
	return cfg, nil
}

// executeAction runs the common tail of runAction/runWrapped: cooldown check,
// step filtering, execution, cooldown recording, logging, and echo output.
func executeAction(cfg config.Config, profile, action string, act *config.Action,
	opts runOpts, vars tmpl.Vars) error {

	cdEnabled, cdSec := resolveCooldown(act, cfg, opts.Cooldown)
	if cdEnabled && cdSec > 0 && cooldown.Check(profile, action, cdSec) {
		if shouldLog(cfg, opts.Log) {
			eventlog.LogCooldown(profile, action, cdSec)
		}
		return nil
	}

	afk := detectAFK(cfg)

	creds := config.MergeCredentials(cfg.Options.Credentials, cfg.Profiles[profile].Credentials)

	desk := cfg.Profiles[profile].Desktop
	filtered := runner.FilterSteps(act.Steps, afk, opts.RunMode, opts.Elapsed)
	err := runner.Execute(filtered, opts.Volume, creds, vars, desk)
	if cdEnabled && cdSec > 0 {
		cooldown.Record(profile, action)
		if shouldLog(cfg, opts.Log) {
			eventlog.LogCooldownRecord(profile, action, cdSec)
		}
	}
	if shouldLog(cfg, opts.Log) {
		eventlog.Log(action, filtered, afk, vars, desk)
	}
	if shouldEcho(cfg, opts.Echo) {
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

// stdinReader is the function used to read stdin metadata. Replaced in tests.
var stdinReader = readStdinJSON

// readStdinJSON reads JSON from stdin when it is piped (not a TTY).
// Returns nil if stdin is a terminal, empty, or not valid JSON.
func readStdinJSON() map[string]interface{} {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil // interactive terminal, not piped
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// stdinVars returns an extraVars callback that injects stdin JSON fields
// into template variables. Returns nil if data is nil (no piped input).
func stdinVars(data map[string]interface{}) func(*tmpl.Vars) {
	if data == nil {
		return nil
	}
	return func(v *tmpl.Vars) {
		// Extract message: prefer last_assistant_message, fall back to message.
		if s, ok := data["last_assistant_message"].(string); ok {
			v.ClaudeMessage = s
		} else if s, ok := data["message"].(string); ok {
			v.ClaudeMessage = s
		}
		if s, ok := data["hook_event_name"].(string); ok {
			v.ClaudeHook = s
		}
		// Re-marshal for {claude_json} — always succeeds since we just unmarshaled it.
		if raw, err := json.Marshal(data); err == nil {
			v.ClaudeJSON = string(raw)
		}
	}
}

// resolveProfile returns the profile to use. When explicit is false (no
// profile argument given), it auto-selects via match rules or falls back
// to "default".
func resolveProfile(cfg config.Config, profile string, explicit bool) string {
	if !explicit {
		return config.MatchProfile(cfg, cwd())
	}
	return profile
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

// sleepForReminder computes the sleep duration from --delay or --at and
// blocks until the target time. Fatals if both are set or --at is unparseable.
func sleepForReminder(delay time.Duration, atStr string, action string) {
	if delay > 0 && atStr != "" {
		fatal("--delay and --at cannot be used together")
	}

	var d time.Duration
	var targetStr string

	if delay > 0 {
		d = delay
		target := time.Now().Add(d)
		targetStr = target.Format("15:04")
	} else {
		now := time.Now()
		target, err := parseTimeToday(atStr, now)
		if err != nil {
			fatal("--at: %v", err)
		}
		d = time.Until(target)
		if d <= 0 {
			// Already past today — schedule for tomorrow.
			target = target.Add(24 * time.Hour)
			d = time.Until(target)
		}
		targetStr = target.Format("15:04")
	}

	fmt.Printf("Reminder: %s in %s (at %s)\n", action, formatDuration(d), targetStr)
	time.Sleep(d)
}

// parseTimeToday parses a time string in 24h ("15:04") or 12h ("3:04PM")
// format and returns it as a time.Time on the same date as ref.
func parseTimeToday(s string, ref time.Time) (time.Time, error) {
	layouts := []string{"15:04", "3:04PM", "3:04pm", "3:04 PM", "3:04 pm", "15:04:05"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return time.Date(ref.Year(), ref.Month(), ref.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, ref.Location()), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q (expected 15:04 or 3:04PM)", s)
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

// handleProtocolURI handles a notify:// protocol activation URI.
// Called when Windows launches the exe via toast click.
func handleProtocolURI(uri string) {
	u, err := url.Parse(uri)
	if err != nil {
		fatal("invalid protocol URI: %v", err)
	}
	if u.Host != "switch" {
		fatal("unknown protocol action: %s", u.Host)
	}
	dStr := u.Query().Get("desktop")
	if dStr == "" {
		fatal("missing desktop parameter in URI")
	}
	d, err := strconv.Atoi(dStr)
	if err != nil || d < 1 || d > 4 {
		fatal("desktop must be 1-4, got %q", dStr)
	}
	// Detach from the console window before switching. Without this,
	// Windows refocuses the console when this process exits, snapping
	// back to the wrong desktop.
	desktop.HideConsole()
	if err := desktop.SwitchTo(d); err != nil {
		fatal("switch desktop: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

// protocolCmd handles the "protocol" subcommand for managing the
// notify:// URI handler registration.
func protocolCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: notify protocol <register|unregister|status>\n")
		os.Exit(1)
	}
	switch args[0] {
	case "register":
		exe, err := os.Executable()
		if err != nil {
			fatal("cannot determine executable path: %v", err)
		}
		if err := desktop.RegisterProtocol(exe); err != nil {
			fatal("register protocol: %v", err)
		}
		fmt.Println("Registered notify:// protocol handler")
	case "unregister":
		if err := desktop.UnregisterProtocol(); err != nil {
			fatal("unregister protocol: %v", err)
		}
		fmt.Println("Unregistered notify:// protocol handler")
	case "status":
		if desktop.IsProtocolRegistered() {
			fmt.Println("Protocol: registered")
		} else {
			fmt.Println("Protocol: not registered")
		}
		if desktop.Available() {
			cur, _ := desktop.Current()
			count, _ := desktop.Count()
			fmt.Printf("Virtual desktops: %d (current: %d)\n", count, cur)
		} else {
			fmt.Println("Virtual desktops: not available (VirtualDesktopAccessor.dll not found)")
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown protocol subcommand: %s\n", args[0])
		os.Exit(1)
	}
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
  notify watch --pid <PID> [options] [profile]
  notify pipe [options] [profile] [--match <pattern> <action>...]
  notify send [--title <title>] <type> <message>
  notify shell-hook <install|uninstall|status> [--shell <shell>] [--threshold <N>]

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json
  --match, -M <pat> <action>  Select action by output pattern (repeatable, run/pipe mode)
  --log, -L              Write invocation to event log
  --echo, -E             Print summary of steps that ran
  --cooldown, -C         Enable per-action cooldown (rate limiting)
  --heartbeat, -H <dur>  Periodic notification during "run" (e.g. 5m, 2m30s)
  --delay, -D <dur>      Sleep before firing (e.g. 5s, 10m, 1h)
  --at, -A <time>        Fire at a specific time (e.g. 14:30, 2:30PM)
  --port, -p <1-65535>   Port for "dashboard" command (default: 8080)
  --open, -O             Open dashboard in a browser window (app mode)
  --protocol <URI>       Handle a notify:// protocol activation (internal)

Commands:
  init                   Interactive config generator (or --defaults for quick setup)
  send <type> <message>  Send a one-off notification (e.g. send telegram "hello")
                         Supported: say, toast, discord, discord_voice, slack,
                         telegram, telegram_audio, telegram_voice
                         --title <title>  Set toast title (toast only)
  pipe [profile]         Read stdin line-by-line, trigger action on pattern match
                         Without --match, every line triggers "ready"
                         {output} = the matched line
  watch --pid <PID>      Watch a running process; notify when it exits
                         Fires "ready" action with {command} = "PID <N>"
  run                    Wrap a command; map exit code or output pattern to action
                         --heartbeat/-H fires the "heartbeat" action periodically
  shell-hook install     Install shell hook for automatic long-command notifications
                         --shell bash|zsh|powershell  Override auto-detected shell
                         --threshold N  Override threshold in seconds (default: 30)
  shell-hook uninstall   Remove shell hook from shell config
  shell-hook status      Check if shell hook is installed
  play [sound|file.wav]  Preview a built-in sound or WAV file (no args lists built-ins)
  test [profile]         Dry-run: show what would happen without sending
  dashboard [--port N]   Local web UI with watch (day/week/month/year/total), history,
           [--open]      config, test, voice, silent tabs. --open for chromeless window
                         Includes /api/trigger REST endpoint for HTTP-based notifications
  startup [--port N]     Register notify:// protocol + start dashboard (combines
          [--open]       "protocol register" and "dashboard" into one command)
  config validate        Check config file for errors
  history [N]            Show last N log entries (default 10)
  history summary [days|all] Show action counts per day (default 7 days)
  history watch          Live dashboard with summary + hourly breakdown (x or Esc to exit)
  history export [days]  Export log entries as JSON (default: all)
  history remove <profile> Remove all entries for a specific profile
  history clean [days]   Remove old entries, keep last N days (no arg = clear all)
  history clear          Delete all log data
  voice generate [--min-uses N]  Generate AI voice files for frequently used voice steps
                         Only generates for texts used >= N times (default: 3 or config)
  voice test [--voice V] [--speed S] [--model M] <text>
                         Generate and play a voice line on the fly
  voice play [text]      Play all cached voices, or one matching text
  voice list             List cached AI voice files
  voice clear            Delete all cached AI voice files
  voice stats [days|all]  Show voice step text usage frequency (default: all)
  protocol register      Register notify:// URI handler (Windows only)
  protocol unregister    Remove notify:// URI handler
  protocol status        Show protocol registration and virtual desktop info
  silent [duration|off]  Suppress all notifications for a duration (e.g. 1h, 30m)
  list, -l, --list       List all profiles and actions
  version, -V           Show version and build date
  help, -h, --help       Show this help message

Config resolution:
  1. --config <path>              (explicit)
  2. notify-config.json next to binary   (portable)
  3. ~/.config/notify/notify-config.json (user default)
  4. Built-in defaults            (zero-config: ready, error, done, attention)

Profile auto-selection:
  When profile is omitted, match rules select a profile by working
  directory ("dir") or environment variable ("env"). First alphabetical
  match wins. Falls back to "default" if none match.

Template variables:
  {profile}, {Profile}, {time}, {Time}, {date}, {Date}, {hostname}
  Run/watch mode: {command}, {duration}, {Duration}, {output}
  Pipe mode: {output} (the matched line)
  Stdin JSON: {claude_message}, {claude_hook}, {claude_json}

  In run mode, {output} contains the last N lines of command output when
  "output_lines" is set in config. In pipe mode, {output} is the matched line.

  When stdin is piped JSON (e.g. from Claude Code hooks), fields are
  auto-extracted: {claude_message} from "last_assistant_message" or
  "message", {claude_hook} from "hook_event_name", {claude_json} is
  the full raw JSON. No flags needed — detection is automatic.
  When logging is enabled, claude_hook and claude_message are recorded
  in the event log and shown in dashboard toast popups.

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
  notify watch --pid 1234           Watch PID 1234, notify when it exits
  notify watch --pid 1234 boss      Watch PID with a specific profile
  notify --delay 5m ready           Remind me in 5 minutes
  notify -D 1h boss attention       Fire "attention" in 1 hour
  notify --at 14:30 ready           Fire "ready" at 2:30 PM today
  notify -A 9:00AM boss done        Fire at 9 AM (tomorrow if past)
  notify run -M FAIL error -M passed ready -- pytest
                                   Select action by output pattern
  tail -f build.log | notify pipe boss -M SUCCESS done -M FAIL error
                                   Pipe mode: match patterns in stream
  deploy-events | notify pipe ops  Every line triggers "ready"

Created by Thomas Häuser
https://mavwarf.netlify.app/
https://github.com/Mavwarf/notify`)
}
