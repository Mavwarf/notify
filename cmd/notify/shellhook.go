package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/shell"
)

// DefaultShellHookThreshold is used when neither config nor --threshold is set.
const DefaultShellHookThreshold = 30

func shellHookCmd(args []string, configPath string) {
	// Parse --shell and --threshold flags.
	shellOverride := ""
	thresholdOverride := -1
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 < len(args) {
				shellOverride = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --shell requires a value (bash, zsh, powershell)\n")
				os.Exit(1)
			}
		case "--threshold":
			if i+1 < len(args) {
				v, err := strconv.Atoi(args[i+1])
				if err != nil || v < 0 {
					fmt.Fprintf(os.Stderr, "Error: --threshold must be a non-negative integer (seconds)\n")
					os.Exit(1)
				}
				thresholdOverride = v
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --threshold requires a value (seconds)\n")
				os.Exit(1)
			}
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: notify shell-hook <install|uninstall|status> [--shell bash|zsh|powershell] [--threshold N]\n")
		os.Exit(1)
	}

	switch rest[0] {
	case "install":
		shellHookInstall(configPath, shellOverride, thresholdOverride)
	case "uninstall":
		shellHookUninstall(shellOverride)
	case "status":
		shellHookStatus(shellOverride)
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell-hook subcommand: %s\n", rest[0])
		fmt.Fprintf(os.Stderr, "Usage: notify shell-hook <install|uninstall|status>\n")
		os.Exit(1)
	}
}

// resolveShell returns the explicit override or auto-detects the current shell.
func resolveShell(override string) string {
	if override != "" {
		return override
	}
	return shell.DetectShell()
}

func shellHookInstall(configPath, shellOverride string, thresholdOverride int) {
	sh := resolveShell(shellOverride)

	// Resolve threshold: CLI flag > config > default.
	threshold := DefaultShellHookThreshold
	if thresholdOverride >= 0 {
		threshold = thresholdOverride
	} else {
		cfg, err := config.Load(configPath)
		if err == nil && cfg.Options.ShellHookThreshold > 0 {
			threshold = cfg.Options.ShellHookThreshold
		}
	}

	// Resolve notify binary path.
	notifyPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine notify binary path: %v\n", err)
		os.Exit(1)
	}
	notifyPath, _ = filepath.Abs(notifyPath)

	configFile, err := shell.Install(sh, threshold, notifyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("notify: shell hook installed in %s (threshold: %ds)\n", configFile, threshold)

	switch sh {
	case "bash":
		fmt.Printf("notify: restart your shell or run: source ~/.bashrc\n")
	case "zsh":
		fmt.Printf("notify: restart your shell or run: source ~/.zshrc\n")
	case "powershell":
		fmt.Printf("notify: restart PowerShell or run: . $PROFILE\n")
	}
}

func shellHookUninstall(shellOverride string) {
	sh := resolveShell(shellOverride)

	configFile, err := shell.Uninstall(sh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("notify: shell hook removed from %s\n", configFile)
}

func shellHookStatus(shellOverride string) {
	sh := resolveShell(shellOverride)

	configFile, installed, err := shell.IsInstalled(sh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if installed {
		fmt.Printf("notify: shell hook is installed in %s\n", configFile)
	} else {
		fmt.Printf("notify: shell hook is not installed (%s)\n", configFile)
	}
}
