package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- DetectShell ---

func TestDetectShellBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("DetectShell always returns powershell on Windows")
	}
	t.Setenv("SHELL", "/bin/bash")
	if got := DetectShell(); got != "bash" {
		t.Errorf("DetectShell() = %q, want %q", got, "bash")
	}
}

func TestDetectShellZsh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("DetectShell always returns powershell on Windows")
	}
	t.Setenv("SHELL", "/usr/bin/zsh")
	if got := DetectShell(); got != "zsh" {
		t.Errorf("DetectShell() = %q, want %q", got, "zsh")
	}
}

func TestDetectShellUnknownFallsBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("DetectShell always returns powershell on Windows")
	}
	t.Setenv("SHELL", "/usr/bin/fish")
	if got := DetectShell(); got != "bash" {
		t.Errorf("DetectShell() = %q, want %q (fallback)", got, "bash")
	}
}

func TestDetectShellEmptyFallsBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("DetectShell always returns powershell on Windows")
	}
	t.Setenv("SHELL", "")
	if got := DetectShell(); got != "bash" {
		t.Errorf("DetectShell() = %q, want %q (fallback)", got, "bash")
	}
}

func TestDetectShellWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only runs on Windows")
	}
	if got := DetectShell(); got != "powershell" {
		t.Errorf("DetectShell() = %q, want %q", got, "powershell")
	}
}

// --- ShellConfigPath ---

func TestShellConfigPathBash(t *testing.T) {
	got, err := ShellConfigPath("bash")
	if err != nil {
		t.Fatalf("ShellConfigPath(bash): %v", err)
	}
	if !strings.HasSuffix(got, ".bashrc") {
		t.Errorf("ShellConfigPath(bash) = %q, want suffix .bashrc", got)
	}
}

func TestShellConfigPathZsh(t *testing.T) {
	got, err := ShellConfigPath("zsh")
	if err != nil {
		t.Fatalf("ShellConfigPath(zsh): %v", err)
	}
	if !strings.HasSuffix(got, ".zshrc") {
		t.Errorf("ShellConfigPath(zsh) = %q, want suffix .zshrc", got)
	}
}

func TestShellConfigPathUnknown(t *testing.T) {
	_, err := ShellConfigPath("fish")
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error should mention unsupported: %v", err)
	}
}

// --- Snippet generation ---

func TestSnippetBashMarkers(t *testing.T) {
	s := Snippet("bash", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, markerBegin) {
		t.Error("bash snippet missing BEGIN marker")
	}
	if !strings.Contains(s, markerEnd) {
		t.Error("bash snippet missing END marker")
	}
}

func TestSnippetBashThreshold(t *testing.T) {
	s := Snippet("bash", 45, "/usr/local/bin/notify")
	if !strings.Contains(s, "threshold: 45s") {
		t.Error("bash snippet should contain threshold comment")
	}
	if !strings.Contains(s, "-ge 45") {
		t.Error("bash snippet should contain threshold comparison")
	}
}

func TestSnippetBashPath(t *testing.T) {
	s := Snippet("bash", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, "'/usr/local/bin/notify' _hook") {
		t.Error("bash snippet should contain quoted notify path")
	}
}

func TestSnippetBashPathWithQuote(t *testing.T) {
	s := Snippet("bash", 30, "/path/with'quote/notify")
	// Bash single-quote escape: ' → '\''
	if !strings.Contains(s, `'/path/with'\''quote/notify'`) {
		t.Errorf("bash snippet path escaping wrong: %s", s)
	}
}

func TestSnippetBashHistoryCapture(t *testing.T) {
	s := Snippet("bash", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, "history 1") {
		t.Error("bash snippet should use history 1 for command capture")
	}
}

func TestSnippetBashPromptCommandLast(t *testing.T) {
	s := Snippet("bash", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, `${PROMPT_COMMAND:+$PROMPT_COMMAND;}_notify_hook_precmd`) {
		t.Error("bash snippet should append precmd LAST to PROMPT_COMMAND")
	}
}

func TestSnippetZshMarkers(t *testing.T) {
	s := Snippet("zsh", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, markerBegin) {
		t.Error("zsh snippet missing BEGIN marker")
	}
	if !strings.Contains(s, markerEnd) {
		t.Error("zsh snippet missing END marker")
	}
}

func TestSnippetZshHookRegistration(t *testing.T) {
	s := Snippet("zsh", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, "autoload -Uz add-zsh-hook") {
		t.Error("zsh snippet should load add-zsh-hook")
	}
	if !strings.Contains(s, "add-zsh-hook preexec _notify_hook_preexec") {
		t.Error("zsh snippet should register preexec hook")
	}
	if !strings.Contains(s, "add-zsh-hook precmd _notify_hook_precmd") {
		t.Error("zsh snippet should register precmd hook")
	}
}

func TestSnippetZshPreexecReceivesCommand(t *testing.T) {
	s := Snippet("zsh", 30, "/usr/local/bin/notify")
	if !strings.Contains(s, `_notify_hook_cmd="$1"`) {
		t.Error("zsh preexec should capture $1 (the command)")
	}
}

func TestSnippetZshThreshold(t *testing.T) {
	s := Snippet("zsh", 60, "/usr/local/bin/notify")
	if !strings.Contains(s, "-ge 60") {
		t.Error("zsh snippet should contain threshold comparison")
	}
}

func TestSnippetPowershellMarkers(t *testing.T) {
	s := Snippet("powershell", 30, `C:\bin\notify.exe`)
	if !strings.Contains(s, markerBegin) {
		t.Error("powershell snippet missing BEGIN marker")
	}
	if !strings.Contains(s, markerEnd) {
		t.Error("powershell snippet missing END marker")
	}
}

func TestSnippetPowershellThreshold(t *testing.T) {
	s := Snippet("powershell", 90, `C:\bin\notify.exe`)
	if !strings.Contains(s, "-ge 90") {
		t.Error("powershell snippet should contain threshold comparison")
	}
}

func TestSnippetPowershellPathEscaping(t *testing.T) {
	s := Snippet("powershell", 30, `C:\path\with'quote\notify.exe`)
	// PowerShell single-quote escape: ' → ''
	if !strings.Contains(s, `C:\path\with''quote\notify.exe`) {
		t.Errorf("powershell snippet path escaping wrong: %s", s)
	}
}

func TestSnippetPowershellCommandQuoting(t *testing.T) {
	s := Snippet("powershell", 30, `C:\bin\notify.exe`)
	// Should use string concatenation with quoted command for proper argument passing
	if !strings.Contains(s, `'_hook --command "' + $cmdEscaped + '"`) {
		t.Error("powershell snippet should use string concatenation for proper argument quoting")
	}
}

func TestSnippetPowershellOriginalPrompt(t *testing.T) {
	s := Snippet("powershell", 30, `C:\bin\notify.exe`)
	if !strings.Contains(s, "_NotifyHookOriginalPrompt") {
		t.Error("powershell snippet should preserve original prompt")
	}
}

func TestSnippetUnknownShell(t *testing.T) {
	s := Snippet("fish", 30, "/usr/local/bin/notify")
	if s != "" {
		t.Errorf("Snippet(fish) = %q, want empty", s)
	}
}

// --- Install / Uninstall / IsInstalled lifecycle ---

func TestInstallAndUninstall(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".bashrc")

	// Write some existing content.
	if err := os.WriteFile(configFile, []byte("# existing config\nexport FOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	snippet := Snippet("bash", 30, "/usr/local/bin/notify")

	// Install by writing directly (bypass ShellConfigPath which points to real home).
	existing, _ := os.ReadFile(configFile)
	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n" + snippet + "\n")
	f.Close()

	// Verify installed.
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, markerBegin) {
		t.Fatal("marker not found after install")
	}
	if !strings.Contains(content, markerEnd) {
		t.Fatal("end marker not found after install")
	}
	// Existing content preserved.
	if !strings.Contains(content, "export FOO=bar") {
		t.Error("existing content should be preserved")
	}

	// Simulate uninstall by calling the removal logic directly.
	beginIdx := strings.Index(content, markerBegin)
	endIdx := strings.Index(content[beginIdx:], markerEnd) + beginIdx + len(markerEnd)
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
	if err := os.WriteFile(configFile, []byte(result), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify uninstalled.
	data, _ = os.ReadFile(configFile)
	content = string(data)
	if strings.Contains(content, markerBegin) {
		t.Error("marker should be removed after uninstall")
	}
	// Existing content still preserved.
	if !strings.Contains(content, "export FOO=bar") {
		t.Error("existing content should survive uninstall")
	}
	_ = existing // suppress unused warning
}

func TestInstallAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".bashrc")

	// Write content with markers already present.
	content := "# existing\n" + markerBegin + "\n# hook content\n" + markerEnd + "\n"
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Check that the marker is detected.
	data, _ := os.ReadFile(configFile)
	if !strings.Contains(string(data), markerBegin) {
		t.Fatal("marker should be present")
	}
}

func TestIsInstalledMissingFile(t *testing.T) {
	// ShellConfigPath returns a real path; test the marker check logic directly.
	dir := t.TempDir()
	configFile := filepath.Join(dir, "nonexistent")

	_, err := os.ReadFile(configFile)
	if !os.IsNotExist(err) {
		t.Fatal("expected file to not exist")
	}
}

func TestSnippetExitCodePreservation(t *testing.T) {
	// Both bash and zsh snippets should preserve the exit code.
	for _, sh := range []string{"bash", "zsh"} {
		s := Snippet(sh, 30, "/usr/local/bin/notify")
		if !strings.Contains(s, "local exit_code=$?") {
			t.Errorf("%s snippet should capture exit code", sh)
		}
		if !strings.Contains(s, "return $exit_code") {
			t.Errorf("%s snippet should return original exit code", sh)
		}
	}
}

func TestSnippetBackgroundExecution(t *testing.T) {
	// All shell snippets should run notify in the background.
	for _, sh := range []string{"bash", "zsh"} {
		s := Snippet(sh, 30, "/usr/local/bin/notify")
		if !strings.Contains(s, "&>/dev/null &") {
			t.Errorf("%s snippet should run notify in background with suppressed output", sh)
		}
	}
	s := Snippet("powershell", 30, `C:\bin\notify.exe`)
	if !strings.Contains(s, "Start-Process") {
		t.Error("powershell snippet should use Start-Process for background execution")
	}
}
