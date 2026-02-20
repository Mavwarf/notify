# TODO

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
