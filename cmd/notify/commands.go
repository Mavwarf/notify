package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/audio"
	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/dashboard"
	"github.com/Mavwarf/notify/internal/desktop"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/procwait"
	"github.com/Mavwarf/notify/internal/runner"
	"github.com/Mavwarf/notify/internal/silent"
	"github.com/Mavwarf/notify/internal/tmpl"
	"github.com/Mavwarf/notify/internal/voice"
)

// sendTypes is the set of step types supported by "notify send".
var sendTypes = map[string]bool{
	"say": true, "toast": true,
	"discord": true, "discord_voice": true,
	"slack": true,
	"telegram": true, "telegram_audio": true, "telegram_voice": true,
}

func sendCmd(args []string, configPath string, opts runOpts) {
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
		fatal("unsupported send type %q\nSupported: say, toast, discord, discord_voice, slack, telegram, telegram_audio, telegram_voice", stepType)
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}

	opts.Volume = resolveVolume(opts.Volume, cfg)

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
	if err := runner.Execute(steps, opts.Volume, cfg.Options.Credentials, vars, nil); err != nil {
		fatal("%v", err)
	}
	if shouldLog(cfg, opts.Log) {
		eventlog.Log("send:"+stepType, steps, false, vars, nil)
	}
	if shouldEcho(cfg, opts.Echo) {
		printEcho(steps)
	}
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
		if cfg, err := loadAndValidate(configPath); err == nil && shouldLog(cfg, logFlag) {
			eventlog.LogSilentDisable()
		}
		return
	}

	d, err := time.ParseDuration(args[0])
	if err != nil {
		fatal("invalid duration %q (examples: 30s, 5m, 1h, 2h30m)", args[0])
	}
	if d <= 0 {
		fatal("duration must be positive")
	}

	silent.Enable(d)
	fmt.Printf("Silent until %s\n", time.Now().Add(d).Format("15:04:05"))
	if cfg, err := loadAndValidate(configPath); err == nil && shouldLog(cfg, logFlag) {
		eventlog.LogSilentEnable(d)
	}
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
		fatal("%v", err)
	}
	if _, err := loadAndValidate(configPath); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("Config OK: %s\n", p)
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
		fatal("%v", err)
	}
}

func listProfiles(configPath string) {
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
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
		if p.Match != nil {
			var parts []string
			if p.Match.Dir != "" {
				parts = append(parts, "dir="+p.Match.Dir)
			}
			if p.Match.Env != "" {
				parts = append(parts, "env="+p.Match.Env)
			}
			annotations = append(annotations, fmt.Sprintf("match: %s", strings.Join(parts, ", ")))
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
		fatal("%v", err)
	}

	if len(args) == 0 {
		profile = config.MatchProfile(cfg, cwd())
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
		fatal("profile %q not found", profile)
	}

	actionNames := make([]string, 0, len(p.Actions))
	for name := range p.Actions {
		actionNames = append(actionNames, name)
	}
	sort.Strings(actionNames)

	// Open voice cache once for dry-run voice source info.
	voiceCache, _ := voice.OpenCache()

	fmt.Printf("\nActions:\n")
	for _, aName := range actionNames {
		act := p.Actions[aName]
		wouldRun := runner.FilteredIndices(act.Steps, afk, false, 0)
		fmt.Printf("\n  %s (%d/%d steps would run):\n", aName, len(wouldRun), len(act.Steps))
		for i, s := range act.Steps {
			marker := "  SKIP "
			if wouldRun[i] {
				marker = "  RUN  "
			}
			detail := eventlog.StepSummary(s, nil)
			voiceSrc := dryRunVoiceSource(s, voiceCache, cfg.Options.Voice.Voice)
			if voiceSrc != "" {
				detail += "  " + voiceSrc
			}
			fmt.Printf("    %s[%d] %-10s %s\n", marker, i+1, s.Type, detail)
		}
	}
}

// dryRunVoiceSource returns a parenthetical voice source label for voice-capable
// step types (say, discord_voice, telegram_audio, telegram_voice).
// Returns "" for non-voice steps.
func dryRunVoiceSource(s config.Step, cache *voice.Cache, voiceName string) string {
	switch s.Type {
	case "say", "discord_voice", "telegram_audio", "telegram_voice":
		// voice-capable step — continue
	default:
		return ""
	}
	if tmpl.HasDynamic(s.Text) {
		return "(system tts, dynamic)"
	}
	if cache != nil {
		if _, ok := cache.Lookup(s.Text); ok {
			name := voiceName
			if name == "" {
				name = config.DefaultVoiceName
			}
			return fmt.Sprintf("(ai: %s)", name)
		}
	}
	return "(system tts)"
}

func credStatus(ok bool) string {
	if ok {
		return " configured"
	}
	return " not configured"
}

func watchCmd(args []string, configPath string, opts runOpts) {
	opts.RunMode = true
	// Parse --pid flag from args.
	pid := -1
	rest := make([]string, len(args))
	copy(rest, args)
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--pid" && i+1 < len(rest) {
			v, err := strconv.Atoi(rest[i+1])
			if err != nil || v <= 0 {
				fatal("--pid requires a positive integer")
			}
			pid = v
			rest = append(rest[:i], rest[i+2:]...)
			break
		}
	}

	if pid < 0 {
		fatal("--pid is required\nUsage: notify watch --pid <PID> [profile]")
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}

	// Remaining args are optional [profile].
	profile := "default"
	explicit := len(rest) > 0
	if explicit {
		profile = rest[0]
	}
	profile = resolveProfile(cfg, profile, explicit)

	fmt.Fprintf(os.Stderr, "notify: watching PID %d...\n", pid)

	start := time.Now()
	if err := procwait.Wait(pid); err != nil {
		fatal("%v", err)
	}
	elapsed := time.Since(start)

	opts.Elapsed = elapsed
	dispatchActions(cfg, profile, "ready", opts,
		func(v *tmpl.Vars) {
			v.Command = fmt.Sprintf("PID %d", pid)
			v.Duration = formatDuration(elapsed)
			v.DurationSay = formatDurationSay(elapsed)
		})
}

func startupCmd(configPath string, port int, open bool) {
	if desktop.IsProtocolRegistered() {
		fmt.Println("Protocol: already registered")
	} else {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot determine executable path: %v\n", err)
		} else if err := desktop.RegisterProtocol(exe); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: register protocol: %v\n", err)
		} else {
			fmt.Println("Registered notify:// protocol handler")
		}
	}
	dashboardCmd(configPath, port, open)
}

func dashboardCmd(configPath string, port int, open bool) {
	cfg, err := loadAndValidate(configPath)
	if err != nil {
		fatal("%v", err)
	}
	p, _ := config.FindPath(configPath)
	if err := dashboard.Serve(cfg, p, port, open, nil); err != nil {
		fatal("%v", err)
	}
}

// hookCmd handles the internal "_hook" command called by shell hook snippets.
// Usage: notify _hook --command <cmd> --seconds <N> --exit <code> [profile]
func hookCmd(args []string, configPath string, opts runOpts) {
	opts.RunMode = true
	var command string
	seconds := 0
	exitCode := 0
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--command":
			if i+1 < len(args) {
				command = args[i+1]
				i++
			}
		case "--seconds":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err == nil {
					seconds = v
				}
				i++
			}
		case "--exit":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err == nil {
					exitCode = v
				}
				i++
			}
		default:
			rest = append(rest, args[i])
		}
	}

	cfg, err := loadAndValidate(configPath)
	if err != nil {
		os.Exit(1) // silent failure — running in background from shell
	}

	// Optional profile argument.
	profile := "default"
	explicit := len(rest) > 0
	if explicit {
		profile = rest[0]
	}
	profile = resolveProfile(cfg, profile, explicit)

	// Determine action from exit code.
	actionArg := resolveExitAction(cfg.Options.ExitCodes, exitCode)

	elapsed := time.Duration(seconds) * time.Second
	opts.Elapsed = elapsed
	dispatchActions(cfg, profile, actionArg, opts,
		func(v *tmpl.Vars) {
			v.Command = command
			v.Duration = formatDuration(elapsed)
			v.DurationSay = formatDurationSay(elapsed)
		})
}
