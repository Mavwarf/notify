# TODO

### Command Wrapper (`notify run`)

Today you have to manually wire notify into your commands:

```bash
make build && notify ready || notify error
```

With `notify run`, you just prefix the command:

```bash
notify run make build
```

It runs the command, captures exit code and duration, then automatically
triggers the `ready` action on success (exit 0) or `error` on failure.
New template variables `{command}` and `{duration}` let you include
context in your notifications — e.g. `"Build finished in {duration}"`.

This removes the friction of remembering to chain notify into every
command and makes it practical for ad-hoc use: just prefix any
long-running command and walk away.

### Quiet Hours

A time-based `"when"` condition so you can suppress loud notifications
at night without changing your config or creating separate profiles:

```json
{ "type": "sound", "sound": "success", "when": "hours:8-22" },
{ "type": "toast", "message": "Done!", "when": "hours:22-8" }
```

Useful when you run late-night builds or have CI hooks that fire
overnight — you still get notified (via toast or remote action), but
without waking up the house. Pairs naturally with AFK detection: you
might be present at 11pm but still prefer silent notifications.

### Cooldown / Rate Limiting

Per-action cooldown to prevent notification spam from watch loops
and file watchers:

```json
"ready": {
  "cooldown_seconds": 30,
  "steps": [...]
}
```

If the same profile+action was triggered within the cooldown window,
the invocation silently exits. Essential for tools like `nodemon`,
`cargo watch`, `fswatch`, or CI pipelines that can trigger dozens of
rebuilds per minute — you want one notification when the build breaks,
not thirty.

### More Remote Notification Actions

Additional step types beyond the existing `discord` webhook support:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `slack`    | POST to Slack webhook                | All (net/http) |
| `telegram` | POST to Telegram Bot API             | All (net/http) |
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord integration.
