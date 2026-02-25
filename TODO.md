# TODO

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

### Dashboard Enhancements

**Medium effort:**
- **Profile pie chart** — SVG pie/donut chart in the Watch tab, placed
  to the right of the summary table, showing percentage share per profile.
  Uses the same data already available in `data.summary.profiles` (name,
  total, pct). Inline SVG with `<path>` arcs, color-coded per profile,
  hover tooltips. No new API work needed

**Larger features:**
- **Timeline view** — visual timeline/gantt showing `run` command
  durations (start → end), not just point events
- **Log file stats** — file size, entry count, oldest entry date

---

## Tech Debt / Cleanup

### Centralize HTTP client with timeout (medium)

Discord, Slack, Telegram, and Webhook packages each use
`http.DefaultClient` or `http.Post()` with no timeout configured. A
hung remote server blocks the step indefinitely. Create a shared
`httputil.Client` with a 30-second timeout and connection pool settings,
used by all remote step packages.

### Multipart form upload duplication (low)

Discord's `SendVoice` and Telegram's `sendFile` both implement
multipart form uploads independently (`bytes.Buffer` + `multipart.Writer`
+ file copy). Could extract a shared helper in `httputil` for building
multipart requests with file attachments.

### `handleWatch()` inline types (low)

Eight struct types (`watchAction`, `watchProfile`, `watchSummary`, etc.)
are defined inline inside `handleWatch()`. Move to package-level types
for reuse, testability, and godoc. Similarly, `jsonEntry` is defined
identically in both `handleHistory()` and `handleEvents()`.

### `silentCmd` bypasses config validation (low)

`silentCmd()` in `commands.go` uses `config.Load()` directly instead of
`loadAndValidate()`. Errors are silently ignored. Should match the
pattern used by all other commands.

### Test platform-specific packages (low)

`idle`, `speech`, and `toast` have no tests. All three shell out to
OS commands (`xprintidle`, `espeak`, `notify-send`, `say`, `osascript`,
PowerShell). Could mock `exec.Command` to verify argument construction
and error handling without real system calls.

### Missing tests for history rendering (low)

`renderHourlyTable()` (113 lines), `historyClean()`, `historyExport()`,
and most command functions in `commands.go` (`sendCmd`, `silentCmd`,
`dryRun`) have no direct unit tests. The hourly table in particular
has enough logic to warrant dedicated test cases.
