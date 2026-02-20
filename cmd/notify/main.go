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
	case "run":
		runWrapped(filtered[1:], configPath, volume)
	default:
		runAction(filtered, configPath, volume)
	}
}

func runAction(args []string, configPath string, volume int) {
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

	// Volume priority: CLI --volume > config default_volume > 100
	if volume < 0 {
		volume = cfg.Options.DefaultVolume
	}

	act, err := config.Resolve(cfg, profile, action)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// AFK detection — fail-open (treat as present on error).
	afk := false
	idleSec, err := idle.IdleSeconds()
	if err == nil {
		afk = idleSec >= float64(cfg.Options.AFKThresholdSeconds)
	}

	vars := tmpl.Vars{Profile: profile}
	filtered := runner.FilterSteps(act.Steps, afk, false)
	err = runner.Execute(act, volume, cfg.Options.Credentials.DiscordWebhook, vars, afk, false)
	eventlog.Log(action, filtered, afk, vars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWrapped(args []string, configPath string, volume int) {
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

	if volume < 0 {
		volume = cfg.Options.DefaultVolume
	}

	act, err := config.Resolve(cfg, profile, action)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	afk := false
	idleSec, err := idle.IdleSeconds()
	if err == nil {
		afk = idleSec >= float64(cfg.Options.AFKThresholdSeconds)
	}

	vars := tmpl.Vars{
		Profile:     profile,
		Command:     strings.Join(cmdArgs, " "),
		Duration:    formatDuration(elapsed),
		DurationSay: formatDurationSay(elapsed),
	}
	filtered := runner.FilterSteps(act.Steps, afk, true)
	err = runner.Execute(act, volume, cfg.Options.Credentials.DiscordWebhook, vars, afk, true)
	eventlog.Log(action, filtered, afk, vars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	os.Exit(exitCode)
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

Commands:
  run                    Wrap a command; notify ready on success, error on failure
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
