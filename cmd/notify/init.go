package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/paths"
)

func initCmd(args []string, configPath string) {
	for _, a := range args {
		if a == "--defaults" {
			initDefaults(configPath)
			return
		}
	}
	initInteractive(configPath)
}

// initDefaults writes the built-in default config to a file without prompts.
func initDefaults(configPath string) {
	path := resolveInitPath(configPath)

	cfg := config.DefaultConfig()
	cfg.Options.Log = true
	cfg.Builtin = false

	if err := writeConfig(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote default config to %s\n", path)
	fmt.Println("Edit it to add channels, credentials, and profiles.")
}

// initInteractive walks the user through an interactive config setup.
func initInteractive(configPath string) {
	scanner := bufio.NewScanner(os.Stdin)
	path := resolveInitPath(configPath)

	// Check for existing config.
	if _, err := os.Stat(path); err == nil {
		if !promptYN(scanner, fmt.Sprintf("%s already exists. Overwrite?", path), false) {
			fmt.Println("Aborted.")
			return
		}
	}

	fmt.Println("notify init — interactive config generator")
	fmt.Println()

	// --- Channels ---
	fmt.Println("Which notification channels do you want to enable?")
	fmt.Println("(Sound + Speech are always included)")
	fmt.Println()

	enableToast := promptYN(scanner, "  Toast popups?", false)
	enableDiscord := promptYN(scanner, "  Discord?", false)
	enableSlack := promptYN(scanner, "  Slack?", false)
	enableTelegram := promptYN(scanner, "  Telegram?", false)
	fmt.Println()

	// --- Credentials ---
	var creds config.Credentials

	if enableDiscord {
		creds.DiscordWebhook = promptLine(scanner, "  Discord webhook URL: ")
		if creds.DiscordWebhook != "" {
			validateDiscordWebhook(creds.DiscordWebhook)
		}
	}
	if enableSlack {
		creds.SlackWebhook = promptLine(scanner, "  Slack webhook URL: ")
	}
	if enableTelegram {
		creds.TelegramToken = promptLine(scanner, "  Telegram bot token: ")
		creds.TelegramChatID = promptLine(scanner, "  Telegram chat ID: ")
		if creds.TelegramToken != "" {
			validateTelegramToken(creds.TelegramToken)
		}
	}
	if enableDiscord || enableSlack || enableTelegram {
		fmt.Println()
	}

	// --- Options ---
	fmt.Println("Options:")
	enableLog := promptYN(scanner, "  Enable logging?", true)
	afkStr := promptLineDefault(scanner, "  AFK threshold seconds", "300")
	afkSec, err := strconv.Atoi(afkStr)
	if err != nil || afkSec < 0 {
		afkSec = config.DefaultAFKThreshold
	}
	fmt.Println()

	// --- Profiles ---
	fmt.Println("Additional profiles (enter blank name to finish):")
	var extraProfiles []initProfile
	for {
		name := promptLine(scanner, "  Profile name: ")
		if name == "" {
			break
		}
		dir := promptLine(scanner, "  Match directory (optional): ")
		extraProfiles = append(extraProfiles, initProfile{name: name, dir: dir})
	}
	fmt.Println()

	// --- Build config ---
	cfg := buildInitConfig(enableToast, enableDiscord, enableSlack, enableTelegram,
		creds, enableLog, afkSec, extraProfiles)

	if err := writeConfig(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// --- Summary ---
	fmt.Printf("Config written to %s\n", path)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  notify test             Dry-run the default profile")
	fmt.Println("  notify config validate  Check for errors")
	fmt.Println("  notify ready            Fire a test notification")
}

// initProfile pairs a profile name with an optional match directory.
type initProfile struct {
	name string
	dir  string
}

// buildInitConfig constructs a Config with the user's selected channels and profiles.
func buildInitConfig(toast, discord, slack, telegram bool,
	creds config.Credentials, log bool, afkSec int,
	extra []initProfile) config.Config {

	actions := map[string]config.Action{
		"ready":     buildAction("success", "{Profile} ready", "Ready!", toast, discord, slack, telegram),
		"error":     buildAction("error", "Something went wrong with {profile}", "Error!", toast, discord, slack, telegram),
		"done":      buildAction("blip", "{Profile} done", "Done!", toast, discord, slack, telegram),
		"attention": buildAction("alert", "{Profile} needs your attention", "Needs attention", toast, discord, slack, telegram),
	}

	profiles := map[string]config.Profile{
		"default": {Actions: actions},
	}

	for _, p := range extra {
		ep := config.Profile{Extends: "default"}
		if p.dir != "" {
			ep.Match = &config.MatchRule{Dir: p.dir}
		}
		profiles[p.name] = ep
	}

	cfg := config.Config{
		Options: config.Options{
			AFKThresholdSeconds: afkSec,
			DefaultVolume:       config.DefaultVolume,
			Log:                 log,
			Credentials:         creds,
		},
		Profiles: profiles,
	}
	return cfg
}

// buildAction creates an action with sound + say steps, plus optional
// channel steps (toast, discord, slack, telegram) with "when": "afk".
func buildAction(sound, sayText, message string, toast, discord, slack, telegram bool) config.Action {
	steps := []config.Step{
		{Type: "sound", Sound: sound},
		{Type: "say", Text: sayText},
	}
	if toast {
		steps = append(steps, config.Step{Type: "toast", Title: "{Profile}", Message: message})
	}
	if discord {
		steps = append(steps, config.Step{Type: "discord", Text: sayText, When: "afk"})
	}
	if slack {
		steps = append(steps, config.Step{Type: "slack", Text: sayText, When: "afk"})
	}
	if telegram {
		steps = append(steps, config.Step{Type: "telegram", Text: sayText, When: "afk"})
	}
	return config.Action{Steps: steps}
}

// resolveInitPath determines where to write the config file.
func resolveInitPath(configPath string) string {
	if configPath != "" {
		return configPath
	}
	// Try next-to-binary first.
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), paths.ConfigFileName)
	}
	// Fall back to user config directory.
	return filepath.Join(paths.DataDir(), paths.ConfigFileName)
}

// writeConfig marshals a Config to JSON and writes it atomically.
func writeConfig(path string, cfg config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data = append(data, '\n')
	return paths.AtomicWrite(path, data)
}

// validateDiscordWebhook checks a Discord webhook URL with a GET request.
func validateDiscordWebhook(url string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not reach Discord webhook: %v\n", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "  Warning: Discord webhook returned status %d\n", resp.StatusCode)
	} else {
		fmt.Println("  Discord webhook OK")
	}
}

// validateTelegramToken checks a Telegram bot token with the getMe API.
func validateTelegramToken(token string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not reach Telegram API: %v\n", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "  Warning: Telegram token returned status %d\n", resp.StatusCode)
	} else {
		fmt.Println("  Telegram token OK")
	}
}

// promptYN asks a yes/no question with a default. Returns true for yes.
func promptYN(scanner *bufio.Scanner, question string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	fmt.Printf("%s %s ", question, hint)
	if !scanner.Scan() {
		return defaultYes
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "" {
		return defaultYes
	}
	return answer == "y" || answer == "yes"
}

// promptLine asks a question and returns the trimmed answer.
func promptLine(scanner *bufio.Scanner, question string) string {
	fmt.Print(question)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// promptLineDefault asks a question with a default value shown in brackets.
func promptLineDefault(scanner *bufio.Scanner, question, defaultVal string) string {
	fmt.Printf("%s [%s]: ", question, defaultVal)
	if !scanner.Scan() {
		return defaultVal
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return defaultVal
	}
	return answer
}
