# TODO

## High Impact

### Pluggable Storage Backend — Phase 3 (WebhookStore)

Phase 1 (Store interface + FileStore) and Phase 2 (SQLiteStore) are complete.
SQLite is the default backend with indexed queries, WAL mode, and auto-migration.

**Remaining implementation:**
- `WebhookStore` — forward all events to an HTTP endpoint for
  external systems (Elasticsearch, Grafana Loki, team dashboards)

### Notification Batching

When running multiple Claude Code sessions, a flood of toasts arrive
simultaneously. A `"batch": "5s"` profile option could collect
notifications within a time window and send a single summary
("3 builds finished: webapp, api, worker") instead of separate popups.

### Incoming Webhook Listener

`notify listen --port 9999` receives webhooks from GitHub Actions,
GitLab CI, Jenkins, etc. and triggers local notifications. The reverse
of sending webhooks out — your CI finishes, your desktop chimes.

**Config:**
```json
{
  "listen": {
    "port": 9999,
    "routes": {
      "/github": { "profile": "ci", "action": "done" },
      "/jenkins": { "profile": "ci", "action": "error" }
    }
  }
}
```

Could also accept a simple JSON body with profile/action fields for
generic use.

## Medium Impact

### Chained Actions (`on_success` / `on_failure`)

Actions that trigger other actions based on step outcomes. E.g.
`"on_failure": "escalate"` would run the `escalate` action if any
step in the current action fails. Enables retry and escalation
patterns without shell scripting.

### History Search

`notify history search "deploy"` to grep past notifications by
profile, action, or step content. The event log already stores all
the data — this just needs a text filter on parsed entries.

### Progress Estimate

During `notify run`, if the same command has been logged before, show
an estimated remaining time in the heartbeat notification based on
historical median duration. E.g. "Still running (~2m left based on
last 5 runs)".

### Sound Themes

`"sound_theme": "gentle"` in config swaps all built-in sound
references to a different generated tone set without editing every
step. One config change, different vibe. Possible themes: default,
gentle, urgent, retro, minimal.

### Step Templates

Reusable named step configurations to reduce config duplication.
Instead of repeating the same Discord webhook setup across 10 profiles:

```json
{
  "templates": {
    "team-discord": { "type": "discord", "text": "{Profile} — {command} finished" }
  },
  "profiles": {
    "webapp": {
      "actions": {
        "done": { "steps": ["team-discord", { "type": "sound" }] }
      }
    }
  }
}
```

Steps can be either an object (current) or a string referencing a
named template.

### Notification Deduplication

If the same profile/action fires multiple times within a short window
(e.g., file watcher, parallel shell hooks), collapse into one
notification. Like cooldown but smarter — it batches the duplicates
and sends a count ("webapp ready ×3") rather than dropping silently.

### Smart Silent Hours (Calendar Integration)

Integrate with OS calendar (Windows COM / macOS EventKit) to
auto-suppress audio during meetings. Remote notifications still fire,
but local sound/speech are muted. A new `when` condition like
`"when": "free"` or `"when": "busy"` based on calendar status.

## Low Impact

### Toast Reply Box

Windows toast XML supports text input fields. A reply box could pipe
user input back to stdin or trigger a follow-up action. E.g. a
"Retry?" button on failure that re-runs the command via protocol URI.

### Notification Analytics

`notify stats` showing insights from historical data: "builds are 15%
faster this week", "most active hours: 10am-12pm", "webapp success
rate: 94%". The event log has all the raw data — this adds
interpretation and trend detection on top.

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

### More Remote Notification Actions

Additional step types beyond `discord`, `slack`, and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli + Java |

## Tech Debt

### Bugs / Correctness

- **`SetRetention` not called in notify-app** — `retention_days` config has
  no effect in the desktop app; event log grows forever.
  Fix: call `eventlog.SetRetention(cfg.Options.RetentionDays)` before
  `OpenDefault` in `cmd/notify-app/main.go`.

- **MQTT client ID always `"notify"`** — two concurrent MQTT steps to the
  same broker cause client eviction. Should use a unique ID per connection
  (e.g. PID or random suffix). `internal/runner/runner.go:314`

- **`historyCmd` reads raw file format, not Store API** — the default
  `notify history N` path uses `ReadContent()` + string split instead of
  `eventlog.Entries()`. Works via a SQLiteStore shim today but will break
  if the reconstructed format ever drifts. Fixing this would also let
  `ReadContent()` be removed from the `Store` interface entirely.
  `cmd/notify/history.go:49-72`

- **`fmt.Sscanf` silently swallows port errors in notify-app** —
  `--port abc` silently defaults to 8811 instead of erroring. Should use
  `strconv.Atoi` with explicit error check like the CLI does.
  `cmd/notify-app/main.go:31`

### Maintainability

- **`go 1.24.0` in go.mod** — should be `go 1.24` (no patch version);
  causes linter and diagnostic warnings. `go.mod:3`

- **`gapThreshold` duplicated** — same `5 * time.Minute` constant defined
  independently in `internal/eventlog/summary.go:191` and
  `internal/dashboard/dashboard.go:764`. Change one and CLI/dashboard
  diverge. Should be a single exported constant in `summary.go`.

- **Desktop limit `4` hardcoded in 3 places** — `main.go:860`,
  `config.go:289`, and toast string. Windows supports up to 20 virtual
  desktops. Should be a single named constant.

- **Various magic numbers** — SSE ticker `2s`, retry delay `2s`, protocol
  sleep `200ms`, default ports `8080`/`8811`, date sentinels `2000`/`2099`.
  Should be named constants.

- **`dashboard.go` is 1361 lines** — HTTP handlers, aggregation logic,
  browser launch, and credential redaction all in one file. Aggregation
  functions (~250 lines) could move to a separate file.

### UX

- **`notify history foo` says "count must be a positive integer"** — should
  say "unknown subcommand" instead. Missing `default` case in
  `cmd/notify/history.go` switch.

### Test Coverage Gaps

- `internal/desktop/` — no tests
- `internal/icon/` — no tests (smoke test for `Draw(64)` would help)
- `cmd/notify/history.go` — export, clean, remove, clear, watch untested
- `cmd/notify/init.go` — `buildInitConfig`, `writeConfig` untested
- `internal/dashboard/` — SSE handler and trigger endpoint not covered

### CI/CD

- **notify-app version not injected** — `build-app` job in `release.yml`
  omits `-X main.version=...`, so the app always reports `dev`.

- **Linux not in CI build-check** — compilation errors only caught at
  release time. Should be added to `ci.yml` matrix.

- **Linux binary requires libasound2** (`CGO_ENABLED=1`) but README doesn't
  mention this runtime dependency.
