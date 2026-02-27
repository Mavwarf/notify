package plugin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Mavwarf/notify/internal/tmpl"
)

// defaultTimeout is used when the step's Timeout field is nil.
const defaultTimeout = 10 * time.Second

// Run executes an external command with NOTIFY_* environment variables.
// The command is run through the system shell (sh -c on Unix, cmd /C on
// Windows). Dynamic data is passed exclusively via env vars — the command
// string is NOT expanded through tmpl.Expand to prevent shell injection.
//
// Timeout behavior:
//   - nil  → 10-second default
//   - 0    → no timeout
//   - >0   → that many seconds
func Run(command, text string, timeoutSec *int, vars tmpl.Vars) error {
	timeout := defaultTimeout
	if timeoutSec != nil {
		if *timeoutSec == 0 {
			timeout = 0
		} else {
			timeout = time.Duration(*timeoutSec) * time.Second
		}
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Env = buildEnv(text, vars)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("plugin %q timed out after %v", command, timeout)
		}
		if stderr.Len() > 0 {
			return fmt.Errorf("plugin %q: %s", command, bytes.TrimSpace(stderr.Bytes()))
		}
		return fmt.Errorf("plugin %q: %w", command, err)
	}
	return nil
}

// buildEnv returns the current process environment augmented with
// NOTIFY_* variables derived from the template vars and expanded text.
// Optional fields are only set when non-empty.
func buildEnv(text string, vars tmpl.Vars) []string {
	env := os.Environ()

	// Always present.
	env = append(env,
		"NOTIFY_PROFILE="+vars.Profile,
		"NOTIFY_HOSTNAME="+vars.Hostname,
		"NOTIFY_TIME="+vars.Time,
		"NOTIFY_DATE="+vars.Date,
	)

	// Present when non-empty.
	if text != "" {
		env = append(env, "NOTIFY_TEXT="+text)
	}
	if vars.Command != "" {
		env = append(env, "NOTIFY_COMMAND="+vars.Command)
	}
	if vars.Duration != "" {
		env = append(env, "NOTIFY_DURATION="+vars.Duration)
	}
	if vars.DurationSay != "" {
		env = append(env, "NOTIFY_DURATION_SAY="+vars.DurationSay)
	}
	if vars.Output != "" {
		env = append(env, "NOTIFY_OUTPUT="+vars.Output)
	}
	if vars.ClaudeMessage != "" {
		env = append(env, "NOTIFY_CLAUDE_MESSAGE="+vars.ClaudeMessage)
	}
	if vars.ClaudeHook != "" {
		env = append(env, "NOTIFY_CLAUDE_HOOK="+vars.ClaudeHook)
	}
	if vars.ClaudeJSON != "" {
		env = append(env, "NOTIFY_CLAUDE_JSON="+vars.ClaudeJSON)
	}

	return env
}
