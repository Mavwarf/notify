package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/Mavwarf/notify/internal/idle"
	"github.com/Mavwarf/notify/internal/runner"
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
	case "list", "-l", "--list":
		listProfiles(configPath)
	default:
		run(filtered, configPath, volume)
	}
}

func run(args []string, configPath string, volume int) {
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

	filtered := runner.FilterSteps(act.Steps, afk)
	err = runner.Execute(act, volume, profile, cfg.Options.Credentials.DiscordWebhook, afk)
	eventlog.Log(profile, action, filtered, afk)
	if err != nil {
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

func printUsage() {
	fmt.Println(`notify - Run notification actions from a config file

Usage:
  notify [options] [profile] <action>

Options:
  --volume, -v <0-100>   Override volume (default: config or 100)
  --config, -c <path>    Path to notify-config.json

Commands:
  list, -l, --list       List all profiles and actions
  help, -h, --help       Show this help message

Config resolution:
  1. --config <path>              (explicit)
  2. notify-config.json next to binary   (portable)
  3. ~/.config/notify/notify-config.json (user default)

Examples:
  notify ready                     Run "ready" from the default profile
  notify default ready             Same as above (explicit default)
  notify boss ready                Run "ready" from the boss profile
  notify -v 50 ready               Run at 50% volume
  notify -c my.json default ready  Use a specific config file`)
}
