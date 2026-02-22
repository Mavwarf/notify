# TODO

### Generic Webhook Step

New `webhook` step type — HTTP POST to an arbitrary URL with the message
as body. Covers ntfy.sh, Pushover, Home Assistant, IFTTT, or any custom
endpoint. One implementation, infinite integrations. Support configurable
headers in credentials for auth tokens. Would make a dedicated `ntfy`
step unnecessary since ntfy is just `POST https://ntfy.sh/topic`.

### Custom Sound Files

Let `"sound"` accept a file path alongside the 7 built-in sound names.
Users can bring their own WAV files. The audio player already handles
PCM — adding WAV file loading is straightforward.

### Profile Inheritance

`"extends": "default"` on a profile so it only needs to override
specific actions instead of redefining everything. Reduces config
duplication when multiple profiles share most steps.

### Retry for Remote Steps

If a remote step (discord, slack, telegram, webhook) fails due to a
network error, retry once after a short delay. Best-effort, no config
needed — sensible default for flaky networks.

### Heartbeat for Long Tasks

With `notify run`, optionally ping every N minutes: "still running
(5m elapsed)...". Useful for 30min+ builds so you know the task
hasn't hung. E.g. `notify run --heartbeat 5m -- make build`.

### Config Bootstrapper (`notify init`)

Interactive config generator. Asks which platforms you want, generates
a starter config with credentials. Lowers the barrier for first-time
setup.

### Exit Code Mapping

`notify run` currently maps exit 0 → `ready`, non-zero → `error`.
Allow custom mappings so specific exit codes trigger different actions,
e.g. exit 2 → `warning` with a different notification pipeline.

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
