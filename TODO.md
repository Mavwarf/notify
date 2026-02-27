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

### Scheduled Reminders (`--delay`, `--at`)

Simple timer that fires a profile action later:
- `notify --delay 5m ready` — fire in 5 minutes
- `notify --at 14:30 ready` — fire at a specific time

Useful for "remind me to check this in 10 minutes". The process
would sleep/wait and then execute the normal notification pipeline.

### Log Rotation / Retention Policy

`"retention": "30d"` in config for automatic cleanup on every write.
Currently `history clean` is manual; unbounded log growth will become
a real problem as the log file is the data source for dashboard
analytics, voice stats, and history queries. Auto-prune entries older
than the configured threshold.

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

### REST Trigger API for `notify-app`

When `notify-app` is running, expose a `/api/trigger` endpoint on the dashboard
server so external tools (Claude hooks, scripts, CI) can fire notifications via
HTTP instead of spawning a new `notify.exe` process.

```
GET  http://127.0.0.1:8811/api/trigger?profile=default&action=ready
POST http://127.0.0.1:8811/api/trigger  {"profile":"default","action":"ready"}
```

**Benefits over CLI invocation:**
- Zero startup latency — config already loaded, eventlog already open
- No process spawn overhead per hook call
- Claude hooks become a simple `curl` one-liner
- Works from any language/tool that can make HTTP requests

**Implementation:**
- Add `handleTrigger` in `internal/dashboard/dashboard.go`
- Reuse `runner.Execute()` with the already-loaded config
- Return JSON response with step results (or errors)
- Optional: accept `volume`, `log`, `echo` params to mirror CLI flags

**Claude hook example:**
```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": { "tool_name": "Bash" },
      "command": "curl -s http://127.0.0.1:8811/api/trigger?profile=default&action=ready"
    }]
  }
}
```

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
