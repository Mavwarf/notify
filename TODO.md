# TODO

### Heartbeat for Long Tasks

With `notify run`, optionally ping every N minutes: "still running
(5m elapsed)...". Useful for 30min+ builds so you know the task
hasn't hung. E.g. `notify run --heartbeat 5m -- make build`.

### Config Bootstrapper (`notify init`)

Interactive config generator. Asks which platforms you want, generates
a starter config with credentials. Lowers the barrier for first-time
setup.

### MQTT Publish

Publish a message to an MQTT broker topic. Useful for home automation —
e.g. flash a desk light when a build finishes, or trigger any
Home Assistant automation via MQTT. Config would need broker URL, topic,
and optional auth in credentials. Payload could use template variables
like other steps.

### More Remote Notification Actions (low priority)

Additional step types beyond `discord`, `slack`, and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli + Java |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord/slack/telegram integration.

### Inline Sound Effects in Voice Messages

Mix TTS speech with sound effects in a single audio message using
template syntax: `"{sound:error} Warning! {sound:alarm.wav} Build failed"`.
The output would stitch segments into one audio file: predefined sound,
TTS speech, WAV file, more TTS — all concatenated and sent as a single
voice message to Telegram/Discord.

**Implementation notes:**
- Parse message into segments (sound refs vs text-to-speech spans)
- Capture TTS output to WAV buffer instead of playing directly
  (Windows SAPI → WAV, macOS `say --output-file`)
- Predefined sounds (error, success, etc.) bundled via Go `embed`
  or resolved from a user sounds directory
- Concatenate PCM/WAV segments — normalize sample rate/format first
- Bigger lift than most features; may benefit from an audio utility package

### `notify watch` (PID or File)

Watch a running process or file for changes: `notify watch --pid 1234`
triggers when the PID exits, `notify watch --file build.log` triggers
when the file is modified. Polling-based with configurable interval.

### Plugin System

External scripts as step types. A `"plugin"` step would run a user
script with template variables as environment variables, enabling
integration with any tool without adding it to the binary.

### Web Dashboard (`notify dashboard`)

Local web UI showing real-time notification history, config editor,
and test buttons. Serves on localhost, reads the event log and config.
Nice for debugging and demoing but not essential.

### Chained Actions (`on_success` / `on_failure`)

Actions that trigger other actions based on step outcomes. E.g.
`"on_failure": "escalate"` would run the `escalate` action if any
step in the current action fails. Enables retry and escalation
patterns without shell scripting.

### Shell Integration (`notify shell-hook`)

Install a precmd/preexec hook into bash, zsh, or PowerShell that
automatically notifies after any command exceeding a time threshold.
No `notify run` wrapping needed — the hook measures elapsed time and
calls notify with the command string and duration.

```bash
notify shell-hook install     # add hook to .bashrc / .zshrc / $PROFILE
notify shell-hook uninstall   # remove it
```

Config option for the threshold: `"shell_hook_threshold": 30` (seconds).

**Use cases:**
- `make build` takes 4 minutes — you get a notification without having
  to remember `notify run -- make build`
- Long-running `git rebase`, `docker build`, `terraform apply` — all
  covered transparently
- New users get value immediately after install without learning
  `notify run` syntax

### Context-Aware Profile Selection

Auto-select a profile based on working directory, git remote, or
environment variable. Removes the most common argument from invocations.

```json
"profiles": {
  "boss": {
    "match": {"dir": "*/work/*"},
    "actions": { ... }
  },
  "personal": {
    "match": {"env": "PROJECT=hobby"},
    "actions": { ... }
  }
}
```

```bash
cd ~/work/api && notify done      # auto-selects "boss"
cd ~/projects && notify done      # auto-selects "personal"
notify --profile boss done        # explicit override still works
```

**Use cases:**
- Work repos always notify the boss channel, personal repos go to
  your private Telegram — no need to type the profile name
- CI environments set `PROJECT=deploy`, matching the right profile
  automatically
- Monorepo with multiple teams: different subdirectories match
  different profiles and notification channels

### Output Capture (`{output}` template variable)

Capture the last N lines of wrapped command output and expose them as
a `{output}` template variable. Optionally trigger different actions
based on output patterns via `--match`.

```bash
# Template variable: last 5 lines of output in the notification
notify run boss -- pytest
# Discord message: "pytest done (2m15s)\n\n3 failed, 47 passed"

# Pattern matching: different actions based on output
notify run boss --match "FAIL" error --match "passed" ready -- pytest
```

Config: `"output_lines": 5` controls how many lines `{output}` keeps.

**Use cases:**
- `pytest` / `go test` — see "3 failed, 47 passed" in the Discord
  message instead of just "command failed"
- `docker build` — the notification includes the final image ID or
  the error message
- `terraform apply` — "Apply complete! Resources: 3 added, 1 changed"
  lands directly in Slack
- Different notification sounds for "all tests passed" vs "failures"
  based on output content

### Pipe / Stream Mode (`notify pipe`)

Read lines from stdin and trigger notifications when patterns match.
Useful for long-running processes you can't wrap with `notify run`.

```bash
tail -f build.log | notify pipe boss --match "BUILD SUCCESS" done --match "FAIL" error
docker compose logs -f | notify pipe ops --match "panic" error
```

Without `--match`, every line triggers the default action (useful for
low-volume streams like deployment events).

**Use cases:**
- CI/CD: pipe build output to notify without installing the binary on
  the CI runner — `ssh ci-server 'tail -f build.log' | notify pipe boss`
- Server monitoring: `journalctl -f -u myapp | notify pipe ops --match "ERROR" error`
- Deployment tracking: `kubectl logs -f deploy/app | notify pipe --match "ready" done`
- File watching: `fswatch src/ | notify pipe dev done` — notify on
  every file change (e.g. trigger a rebuild notification)

---

## Tech Debt / Cleanup

### Test platform-specific packages (low)

`idle`, `speech`, and `toast` have no tests. All three shell out to
OS commands (`xprintidle`, `espeak`, `notify-send`, `say`, `osascript`,
PowerShell). Could mock `exec.Command` to verify argument construction
and error handling without real system calls.
