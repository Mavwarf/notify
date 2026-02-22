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

### Notification Groups

Fire multiple actions in one call: `notify boss done,attention` triggers
both the `done` and `attention` actions in sequence. Useful for combining
a notification with an escalation in a single invocation without shell
chaining.

### `notify watch` (PID or File)

Watch a running process or file for changes: `notify watch --pid 1234`
triggers when the PID exits, `notify watch --file build.log` triggers
when the file is modified. Polling-based with configurable interval.

### Conditional Credentials

Per-profile credential overrides so different profiles can post to
different Discord channels or Telegram chats. Currently credentials are
global; this would allow `"credentials"` inside a profile to override
specific fields.

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
