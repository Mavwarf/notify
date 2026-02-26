# TODO

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

### Modern Toast Notifications (Windows)

Upgrade from `NotifyIcon.BalloonTip` to the Windows 10+ `ToastNotificationManager`
XML API. This enables rich text formatting via `hint-style` on `<text>` elements
inside `<group><subgroup>` blocks — including bold (`base`), subtitle, title,
header sizes, and subtle (60% opacity) variants. Could also support hero images,
attribution text, action buttons, and progress bars.

Would require rewriting `internal/toast/toast_windows.go` to build XML templates
and use `Windows.UI.Notifications.ToastNotificationManager` via PowerShell.
Linux and macOS toast implementations would remain unchanged.

### `notify watch` — File Watching (stretch)

PID watching is done (`notify watch --pid <PID>`). File watching
(`notify watch --file build.log`) remains a possibility — trigger when
a file is modified. Would need `fsnotify` or polling with `os.Stat`.

### Plugin System

External scripts as step types. A `"plugin"` step would run a user
script with template variables as environment variables, enabling
integration with any tool without adding it to the binary.

### Chained Actions (`on_success` / `on_failure`)

Actions that trigger other actions based on step outcomes. E.g.
`"on_failure": "escalate"` would run the `escalate` action if any
step in the current action fails. Enables retry and escalation
patterns without shell scripting.

