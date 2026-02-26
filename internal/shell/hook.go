package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	markerBegin = "# BEGIN notify shell-hook"
	markerEnd   = "# END notify shell-hook"
)

// DetectShell returns the current shell type: "bash", "zsh", or "powershell".
// On Windows it defaults to "powershell". On Unix it inspects $SHELL.
func DetectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	sh := os.Getenv("SHELL")
	if strings.HasSuffix(sh, "/zsh") {
		return "zsh"
	}
	if strings.HasSuffix(sh, "/bash") {
		return "bash"
	}
	// Fallback: try basename.
	base := filepath.Base(sh)
	switch base {
	case "zsh":
		return "zsh"
	case "bash":
		return "bash"
	}
	return "bash" // default
}

// ShellConfigPath returns the config file path for the given shell.
func ShellConfigPath(shell string) (string, error) {
	switch shell {
	case "bash":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, ".zshrc"), nil
	case "powershell":
		return psProfilePath()
	default:
		return "", fmt.Errorf("unsupported shell %q (use bash, zsh, or powershell)", shell)
	}
}

// psProfilePath resolves the PowerShell $PROFILE path.
func psProfilePath() (string, error) {
	// Try pwsh first (PowerShell Core), then powershell (Windows PowerShell).
	for _, exe := range []string{"pwsh", "powershell"} {
		out, err := exec.Command(exe, "-NoProfile", "-Command", "$PROFILE").Output()
		if err == nil {
			p := strings.TrimSpace(string(out))
			if p != "" {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("cannot determine PowerShell $PROFILE path")
}

// Snippet returns the shell hook snippet for the given shell.
// threshold is the minimum command duration in seconds before notifying.
// notifyPath is the absolute path to the notify binary.
func Snippet(shell string, threshold int, notifyPath string) string {
	switch shell {
	case "bash":
		return bashSnippet(threshold, notifyPath)
	case "zsh":
		return zshSnippet(threshold, notifyPath)
	case "powershell":
		return powershellSnippet(threshold, notifyPath)
	default:
		return ""
	}
}

// Install appends the shell hook snippet to the shell's config file.
// Returns an error if the hook is already installed.
func Install(shell string, threshold int, notifyPath string) (string, error) {
	configPath, err := ShellConfigPath(shell)
	if err != nil {
		return "", err
	}

	// Check for existing installation.
	existing, _ := os.ReadFile(configPath)
	if strings.Contains(string(existing), markerBegin) {
		return configPath, fmt.Errorf("shell hook already installed in %s (use 'notify shell-hook uninstall' first)", configPath)
	}

	snippet := Snippet(shell, threshold, notifyPath)
	if snippet == "" {
		return "", fmt.Errorf("unsupported shell %q", shell)
	}

	// Append to config file (create if needed).
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return configPath, fmt.Errorf("cannot write to %s: %w", configPath, err)
	}
	defer f.Close()

	// Ensure we start on a new line.
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return configPath, err
		}
	}

	if _, err := f.WriteString("\n" + snippet + "\n"); err != nil {
		return configPath, err
	}

	return configPath, nil
}

// Uninstall removes the marker-delimited shell hook block from the config file.
func Uninstall(shell string) (string, error) {
	configPath, err := ShellConfigPath(shell)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return configPath, fmt.Errorf("shell hook not installed (%s does not exist)", configPath)
		}
		return configPath, err
	}

	content := string(data)
	beginIdx := strings.Index(content, markerBegin)
	if beginIdx < 0 {
		return configPath, fmt.Errorf("shell hook not installed in %s (marker not found)", configPath)
	}

	endIdx := strings.Index(content[beginIdx:], markerEnd)
	if endIdx < 0 {
		return configPath, fmt.Errorf("malformed shell hook in %s (begin marker without end marker)", configPath)
	}
	endIdx += beginIdx + len(markerEnd)

	// Remove the block and any surrounding blank lines.
	before := strings.TrimRight(content[:beginIdx], "\n")
	after := strings.TrimLeft(content[endIdx:], "\n")

	var result string
	if before == "" {
		result = after
	} else if after == "" {
		result = before + "\n"
	} else {
		result = before + "\n\n" + after
	}

	if err := os.WriteFile(configPath, []byte(result), 0644); err != nil {
		return configPath, err
	}

	return configPath, nil
}

// IsInstalled checks whether the shell hook markers are present in the config file.
func IsInstalled(shell string) (configPath string, installed bool, err error) {
	configPath, err = ShellConfigPath(shell)
	if err != nil {
		return "", false, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return configPath, false, nil
		}
		return configPath, false, err
	}

	return configPath, strings.Contains(string(data), markerBegin), nil
}

func bashSnippet(threshold int, notifyPath string) string {
	// Escape the notify path for embedding in bash single quotes.
	escaped := strings.ReplaceAll(notifyPath, "'", "'\\''")
	return fmt.Sprintf(`%s
# Automatic notifications for long-running commands (threshold: %ds)
_notify_hook_cmd=""
_notify_hook_start=""

_notify_hook_preexec() {
    # Skip our own functions (DEBUG trap fires for PROMPT_COMMAND components too)
    [[ "$BASH_COMMAND" == _notify_hook_* ]] && return
    # Only capture once per command cycle (reset in precmd)
    [[ -n "$_notify_hook_start" ]] && return
    _notify_hook_cmd=$(HISTTIMEFORMAT='' history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    _notify_hook_start=$SECONDS
}
trap '_notify_hook_preexec' DEBUG

_notify_hook_precmd() {
    local exit_code=$?
    if [[ -n "$_notify_hook_start" ]] && [[ -n "$_notify_hook_cmd" ]]; then
        local elapsed=$(( SECONDS - _notify_hook_start ))
        if [[ $elapsed -ge %d ]]; then
            '%s' _hook --command "$_notify_hook_cmd" --seconds "$elapsed" --exit "$exit_code" &>/dev/null &
        fi
    fi
    _notify_hook_cmd=""
    _notify_hook_start=""
    return $exit_code
}
PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND;}_notify_hook_precmd"
%s`, markerBegin, threshold, threshold, escaped, markerEnd)
}

func zshSnippet(threshold int, notifyPath string) string {
	escaped := strings.ReplaceAll(notifyPath, "'", "'\\''")
	return fmt.Sprintf(`%s
# Automatic notifications for long-running commands (threshold: %ds)
_notify_hook_cmd=""
_notify_hook_start=""

_notify_hook_preexec() {
    _notify_hook_cmd="$1"
    _notify_hook_start=$SECONDS
}

_notify_hook_precmd() {
    local exit_code=$?
    if [[ -n "$_notify_hook_start" ]] && [[ -n "$_notify_hook_cmd" ]]; then
        local elapsed=$(( SECONDS - _notify_hook_start ))
        if [[ $elapsed -ge %d ]]; then
            '%s' _hook --command "$_notify_hook_cmd" --seconds "$elapsed" --exit "$exit_code" &>/dev/null &
        fi
    fi
    _notify_hook_cmd=""
    _notify_hook_start=""
    return $exit_code
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec _notify_hook_preexec
add-zsh-hook precmd _notify_hook_precmd
%s`, markerBegin, threshold, threshold, escaped, markerEnd)
}

func powershellSnippet(threshold int, notifyPath string) string {
	escaped := strings.ReplaceAll(notifyPath, "'", "''")
	return fmt.Sprintf(`%s
# Automatic notifications for long-running commands (threshold: %ds)
$global:_NotifyHookCmd = $null
$global:_NotifyHookStart = $null

# Capture command text before execution
Set-PSReadLineKeyHandler -Key Enter -ScriptBlock {
    $line = $null
    $cursor = $null
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
    if ($line.Trim()) {
        $global:_NotifyHookCmd = $line.Trim()
        $global:_NotifyHookStart = Get-Date
    }
    [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
}

# Store the original prompt
if (-not (Test-Path Function:\global:_NotifyHookOriginalPrompt)) {
    Copy-Item Function:\prompt Function:\global:_NotifyHookOriginalPrompt
}

function global:prompt {
    $exitCode = if ($?) { 0 } else { $LASTEXITCODE }
    if ($null -eq $exitCode) { $exitCode = 0 }
    if ($null -ne $global:_NotifyHookCmd -and $null -ne $global:_NotifyHookStart) {
        $elapsed = [int]((Get-Date) - $global:_NotifyHookStart).TotalSeconds
        if ($elapsed -ge %d) {
            $cmdEscaped = $global:_NotifyHookCmd -replace '"', '\"'
            Start-Process -NoNewWindow -FilePath '%s' -ArgumentList ('_hook --command "' + $cmdEscaped + '" --seconds ' + $elapsed + ' --exit ' + $exitCode) -RedirectStandardOutput NUL -RedirectStandardError NUL
        }
        $global:_NotifyHookCmd = $null
        $global:_NotifyHookStart = $null
    }
    _NotifyHookOriginalPrompt
}
%s`, markerBegin, threshold, threshold, escaped, markerEnd)
}
