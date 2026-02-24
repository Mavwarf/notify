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

---

## Tech Debt / Cleanup

### Add tests for history commands (medium)

No test coverage for `historySummary`, `historyClean`, `historyExport`,
`historyWatch`, `renderSummaryTable`, or `buildBaseline`. Add table-
driven tests, especially for the table renderer.

### Surface all parallel step errors (medium)

`runner.Execute()` collects errors from parallel steps but only
returns the first one. When multiple remote steps fail (e.g. Discord
is down *and* Telegram token expired), the user only sees one error.
Return or log all failures.

### `builtinSounds` can drift from `audio.Sounds` (low)

`config.go` hardcodes `builtinSounds` to avoid importing the audio
package. If a sound is added to `audio.Sounds` but not mirrored here,
config validation won't recognize it. Consider exporting a
`audio.BuiltinNames()` helper or adding a test that asserts the two
sets are equal.
