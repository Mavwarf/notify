# History

## Features

- Dashboard voice playback — play button in Voice tab to preview pre-generated AI voice files in the browser *(Feb 26)*
- AI voice generation (`notify voice generate/list/clear`) — pre-generate high-quality AI voice files via OpenAI TTS for frequently used say steps; cached WAVs play automatically, falls back to system TTS *(Feb 26)*
- Interactive config generator (`notify init`) — walk-through setup for channels, credentials, and profiles; `--defaults` for quick start *(Feb 26)*
- Dashboard time-spent fix — total now uses merged timeline so overlapping profiles don't inflate wall-clock time *(Feb 26)*
- Shell hook (`notify shell-hook`) — automatic notifications for long-running commands via bash/zsh/PowerShell hooks *(Feb 26)*
- PID watch (`notify watch --pid`) — watch a running process, notify when it exits *(Feb 26)*
- Built-in default config — zero-config fallback with `ready`, `error`, `done`, `attention` actions using local audio *(Feb 26)*
- Stdin JSON injection — auto-detect piped JSON on stdin for hook integration (`{claude_message}`, `{claude_hook}`, `{claude_json}`) *(Feb 25)*
- Voice stats (`notify voice stats`) — say step text usage frequency from event log *(Feb 25)*
- Tests for `renderHourlyTable` — basic, single-profile, empty, single-hour, and gap-hour scenarios *(Feb 25)*
- Web dashboard (`notify dashboard`) — local web UI with watch, history, config viewer, dry-run testing, voice stats, silent mode control, day navigation, log-extracted profiles, credential health check, history filtering, keyboard shortcuts, activity chart, dark/light theme toggle, history export, clickable profile detail view, approximate time spent per profile, profile donut charts, hourly bar chart, activity timeline, log file stats, screenshot mode, and `--open` flag for chromeless browser window *(Feb 25)*
- Heartbeat for long tasks (`--heartbeat`) — periodic notifications during `notify run` *(Feb 24)*
- Pipe / stream mode (`notify pipe`) — trigger notifications from stdin patterns *(Feb 24)*
- Output capture (`{output}`) and pattern matching (`--match`) for `notify run` *(Feb 24)*
- Profile auto-selection — match rules auto-select profile by working directory or env var *(Feb 24)*
- History watch (`notify history watch`) — live-updating today's summary dashboard with hourly breakdown *(Feb 24)*
- Color summary table — profile grouping, percentage column, dynamic columns, ANSI colors, `NO_COLOR` support *(Feb 24)*
- History clean (`notify history clean`) — prune old log entries by age *(Feb 24)*
- Per-profile credential overrides — different profiles can target different channels *(Feb 24)*
- Notification groups — comma-separated actions in a single call *(Feb 23)*
- Direct send (`notify send`) — one-off notifications without a profile *(Feb 23)*
- Profile aliases — shorthand names for profiles *(Feb 22)*
- Template variables: `{time}`, `{Time}`, `{date}`, `{Date}`, `{hostname}` *(Feb 22)*
- Config validate command (`notify config validate`) *(Feb 22)*
- History export (`notify history export`) — JSON export of log entries *(Feb 22)*
- History summary (`notify history summary`) — aggregated usage stats per day *(Feb 22)*
- Profile inheritance (`"extends"`) — inherit actions from a parent profile *(Feb 22)*
- Exit code mapping for `notify run` — map specific codes to custom actions *(Feb 22)*
- Custom sound files — use your own WAV files in `sound` steps *(Feb 22)*
- Generic webhook step for ntfy.sh, Pushover, Home Assistant, IFTTT, etc. *(Feb 22)*
- Automatic retry for remote steps (discord, slack, telegram) *(Feb 22)*
- Environment variables in credentials (`$VAR` / `${VAR}`) *(Feb 22)*
- Sound preview (`notify play`) to audition built-in sounds *(Feb 22)*
- Notification history (`notify history`) to view recent log entries *(Feb 22)*
- Echo option (`--echo`) to print execution summary *(Feb 22)*
- Silent mode (`notify silent`) for temporary suppression *(Feb 22)*
- Remote notifications: Discord, Slack, Telegram, voice messages, voice bubbles *(Feb 20–22)*
- Cooldown / rate limiting per action *(Feb 21)*
- Config validation with multi-error reporting *(Feb 21)*
- Command wrapper (`notify run`) with exit code and duration templates *(Feb 20)*
- Quiet hours — suppress steps outside a time window *(Feb 20)*
- AFK detection — different steps when present vs away *(Feb 20)*
- Template variables: `{profile}`, `{command}`, `{duration}` *(Feb 20)*
- Opt-in event logging *(Feb 20)*
- Multi-step notification pipelines: sound, speech, toast *(Feb 19)*

---

## 2026-02-26

### Dashboard: Voice Playback

The Voice tab now shows a play button next to any say-step text that has a
pre-generated AI voice file in the cache. Clicking the button streams the
cached WAV through the browser's audio API. Clicking again stops playback.
The button tooltip shows the voice name (e.g. "nova"). Backed by a new
`/api/voice/play/{hash}` endpoint that serves WAV files from the voice cache,
and extended `/api/voice` response with `cached`, `hash`, and `voice` fields.

### AI Voice Generation (`notify voice generate/list/clear`)

Pre-generate high-quality AI voice files for frequently used `say` step messages
via the OpenAI TTS API. The `generate` command scans the event log, identifies
messages used at least N times (default 3, configurable via `--min-uses` or
`"min_uses"` in config), and calls the API to create cached WAV files. Messages
with dynamic template variables (`{duration}`, `{time}`, etc.) are skipped and
always fall back to system TTS. When a cached voice exists, the runner plays it
directly through the audio pipeline instead of using system TTS. Cached voices
also benefit remote voice steps (`discord_voice`, `telegram_audio`,
`telegram_voice`). The `list` command shows all cached files, and `clear`
removes them. The `notify test` dry-run shows voice source per say step.

Config:
```json
"openai_voice": { "model": "tts-1", "voice": "nova", "speed": 1.0, "min_uses": 3 },
"credentials": { "openai_api_key": "$OPENAI_API_KEY" }
```

### Interactive Config Generator (`notify init`)

Walk-through setup that generates a complete `notify-config.json`. Prompts for
notification channels (toast, Discord, Slack, Telegram), credentials, options
(logging, AFK threshold), and additional profiles with match rules. Validates
Discord webhooks (GET) and Telegram tokens (`getMe` API) during setup. Use
`notify init --defaults` to write the built-in default config to a file for
manual editing.

### Dashboard: Time-Spent Total Fix

The "Approx. time spent" total in the Watch tab previously summed per-profile
seconds independently. When multiple profiles had activity during the same
time window, the total exceeded actual wall-clock time (e.g. two profiles both
active from 10:00–10:04 would report 8 minutes instead of 4). The total now
uses a merged timeline of all profiles' timestamps with the same 5-minute gap
logic, so overlapping activity is only counted once. Per-profile breakdowns
are unchanged.

### Shell Hook (`notify shell-hook`)

Install a precmd/preexec hook into bash, zsh, or PowerShell that automatically
triggers a notification after any command exceeding a time threshold (default 30s).
No `notify run` wrapping needed — the hook measures elapsed time and calls
`notify _hook` in the background with the command string, duration, and exit code.

```bash
notify shell-hook install              # auto-detect shell, install hook
notify shell-hook install --shell zsh  # explicit shell
notify shell-hook install --threshold 60  # custom threshold (60s)
notify shell-hook uninstall            # clean removal
notify shell-hook status               # check if installed
```

The threshold is embedded directly in the shell snippet as an arithmetic check,
so no Go process spawns for short commands. Exit code mapping (`exit_codes` in
config) works automatically — the snippet captures `$?` / `$LASTEXITCODE` and
passes it to `notify _hook --exit`.

Config option `"shell_hook_threshold"` sets the default threshold (used when
`--threshold` is not specified at install time). Snippets are delimited by
`# BEGIN notify shell-hook` / `# END notify shell-hook` markers for clean
uninstall.

### PID Watch (`notify watch --pid`)

Watch a running process and fire a notification when it exits. Useful when you
started a long command and forgot to wrap it with `notify run`:

```bash
notify watch --pid 1234           # default profile, "ready" action
notify watch --pid 1234 boss      # specific profile
```

Uses platform-specific efficient waiting: `OpenProcess(SYNCHRONIZE)` +
`WaitForSingleObject` on Windows (true kernel-level blocking, no polling),
`kill(pid, 0)` polling every 500ms on Linux/macOS. Template variables
`{command}` (set to `"PID <N>"`), `{duration}`, and `{Duration}` are available.
No exit code detection — non-child processes don't expose exit codes
cross-platform, so the action is always `ready`.

### Built-in Default Config (Zero-Config Fallback)
When no `notify-config.json` exists and no `--config` path is specified, `notify`
now falls back to a built-in default configuration instead of erroring with "no
notify-config.json found". The built-in config provides a `default` profile with
four actions: `ready` (success sound + "{Profile} ready"), `error` (error sound +
"Something went wrong with {profile}"), `done` (blip sound + "{Profile} done"),
and `attention` (alert sound + "{Profile} needs your attention"). All steps use
local audio only — no credentials, no logging, no cooldown. A one-line hint is
printed to stderr (`notify: using built-in defaults (create notify-config.json to
customize)`) so users know they can create a config file for full customization.
If `--config` points to a non-existent path, the error is still raised. Once a
config file exists, the built-in default is completely ignored.

---

## 2026-02-25

### Stdin JSON injection for hook integration
When stdin is piped (not a terminal), `notify` auto-detects JSON input and
extracts fields as template variables: `{claude_message}` from
`last_assistant_message` or `message`, `{claude_hook}` from `hook_event_name`,
and `{claude_json}` for the full raw JSON. Designed for seamless integration
with Claude Code hooks — the hook command stays unchanged, and notification
steps can reference the AI's message via `{claude_message}`. When stdin is a
terminal or not valid JSON, the variables expand to empty strings.

Event log summary lines now include `claude_hook=` and `claude_message=`
fields when present, so you can see which hook triggered each notification
and what message was passed. The web dashboard picks these up too — live
toast popups show the hook source (e.g. "via Stop") and the claude message
text when available. Toasts with messages stay visible for 6 seconds instead
of the default 4.

### Voice Stats (`notify voice stats`)
New subcommand that scans the event log for `say` step texts and shows usage
frequency. Helps identify which voice lines are actually used before generating
AI voice files. Supports optional `[days|all]` argument to filter by time range
(default: all time). Output includes rank, count, percentage, and the quoted
text. Backed by a new `ParseVoiceLines()` function in the eventlog package.

### Web Dashboard (`notify dashboard`)
Local web UI served on `http://127.0.0.1:8080` (configurable with `--port`).
Six tabs: **Watch** (default) mirrors the terminal `history watch` output
with a summary table showing per-profile/action counts, percentages, skipped
entries, and "New" deltas since page load, plus an hourly breakdown table
with bar chart —
all auto-refreshing every 2 seconds. **History** shows a live-updating table
of notification events fed by Server-Sent Events. **Config** displays a
read-only JSON view of the loaded config with credentials redacted to `"***"`.
**Test** provides a dry-run interface where you pick a profile and action to
see which steps would run without actually sending anything — the profile
dropdown merges config profiles with profiles extracted from the last 48h of
log entries, and unknown profiles fall back to the `default` profile using
the same `Resolve()` logic as the CLI. Template variables (`{profile}`,
`{time}`, etc.) are expanded in step details. **Voice** shows say-step text
frequencies from the event log with rank, count, percentage, and text columns;
a time-range dropdown filters by all time, 7, 30, or 90 days. The **Watch**
tab supports day
navigation with prev/next/today buttons and accepts a `?date=YYYY-MM-DD`
query param; the "New" column only appears when viewing today. Tabs are
linkable via URL hash (e.g. `/#watch`, `/#history`). The dashboard uses
`go:embed` to bundle a single self-contained HTML file (dark theme, vanilla
JS, no dependencies) into the binary. API endpoints: `/api/watch` (summary +
hourly JSON, optional `?date=`), `/api/config`, `/api/history`,
`/api/summary`, `/api/events` (SSE), `/api/test` (dry-run with fallback),
`/api/stats` (log file metadata), `/api/voice` (say-step text frequencies),
`/api/silent` (GET status, POST enable/disable), `/api/credentials`.
Config is loaded once at startup. Binds to localhost only. Added
`Profile.MarshalJSON()` to enable JSON serialization of profiles.
Press `Ctrl+C` to stop.

### Dashboard: Credential Health Check
The **Config** tab now shows a credential health panel above the JSON viewer.
For each profile that uses remote step types (discord, slack, telegram), the
panel displays colored badges showing whether the required credentials are
present ("ok" in green) or missing ("missing" in red). Credentials are merged
using the same global + profile override logic as the CLI. Profiles that only
use local steps (sound, say, toast) are omitted. Backed by `/api/credentials`.

### Dashboard: History Filtering
The **History** tab now has profile and kind filter dropdowns above the event
table. Filter by any profile seen in the loaded entries, and/or by event kind
(execution, cooldown, silent). Filters apply to both the initial load and new
SSE entries arriving in real time. Profile options are populated dynamically
from the loaded data.

### Dashboard: Keyboard Shortcuts
Press `1`–`4` to switch between Watch, History, Config, and Test tabs. On the
Watch tab, use left/right arrow keys to navigate days, and `t` to jump to
today. Shortcuts are disabled when a form input is focused. A hint line below
the header shows the available keys.

### Dashboard: Activity Chart
The **History** tab now shows an SVG bar chart between the Summary table and the
Logs table. Each bar represents one day with stacked segments: green for
executions, yellow for skipped. Hover any bar for a tooltip with the exact date
and counts. The chart reuses the existing "Show last" dropdown to control its
time range and automatically hides for hour-based ranges (1h, 4h, 12h) since
sub-day granularity doesn't apply to a daily chart. Y-axis grid lines scale
dynamically to the data. No extra API calls — built from the same
`/api/summary` response used by the Summary table.

### Dashboard: Dark/Light Theme Toggle
A sun/moon toggle button in the header switches between dark (Tokyo Night) and
light themes. All colors are driven by CSS variables — the light theme overrides
`--bg`, `--fg`, `--accent`, and all semantic colors with higher-contrast values
suited for white backgrounds. Badge and credential indicator backgrounds use
adjusted rgba values for readability on light surfaces. The activity chart reads
colors from CSS variables at render time so bars and grid lines follow the theme.
Preference is saved to `localStorage` and restored on page load. Dark is the
default.

### Dashboard: History Export
CSV and JSON export buttons in the History tab let you download the currently
loaded and filtered entries. The buttons sit next to the "Show last" dropdown
and respect active profile/kind filters — only what you see in the table gets
exported. CSV includes a `Time,Profile,Action,Kind` header row with proper
quoting for values containing commas or quotes. JSON outputs a pretty-printed
array matching the `notify history export` format. Files are named
`notify-history-YYYY-MM-DD.csv` / `.json`. Download is client-side via Blob
URL — no server round-trip needed.

### Dashboard: Profile Detail View
Click any profile name across the dashboard (Watch, History, Config, Test tabs)
to open a detail modal showing the profile's full step pipeline and credential
status. The modal fetches dry-run data from `/api/test` (all actions) and
displays each action with RUN/SKIP markers, step types, and expanded template
details — the same rendering used by the Test tab. If the profile uses remote
steps, credential health badges (ok/missing) appear below the actions. Credential
data is cached from the initial page load to avoid redundant requests. Close the
modal via the X button, clicking the backdrop, or pressing Escape.

### Dashboard: Approximate Time Spent
The **Watch** tab now shows an "Approx. time spent" section between the summary
table and the hourly breakdown. For each profile, consecutive log entries are
walked chronologically — if the gap between two entries is 5 minutes or less,
that gap is counted as active time. Gaps exceeding 5 minutes are treated as
idle breaks and ignored. Each profile row displays the estimated time in
`Xh Ym` format with a percentage column, plus a total row. The total is
computed from a merged timeline of all profiles so overlapping activity windows
are counted once (not double-counted per profile). The section is hidden when
there is no estimated active time (e.g. profiles with only a single entry per
session). Computed server-side in the `/api/watch` response.

### Dashboard: Charts & Activity Timeline
The **Watch** tab displays outline-style charts next to each data table: donut
charts beside the summary and time-spent tables showing notification share and
time distribution per profile, a vertical bar chart beside the hourly
breakdown showing activity by hour, and an activity timeline heatmap below it
showing a row per profile with cells for each hour — opacity indicates
intensity. Hover any element for a tooltip. All charts use the foreground
color for outlines, adapting to dark/light theme automatically.

### Dashboard: Screenshot Mode
Press `S` to toggle screenshot mode, which replaces real profile names with
fake ones (project-alpha, project-beta, etc.) across all tabs — Watch, History,
Config, and Test. Useful for taking screenshots for blog posts or documentation
without exposing real project names. The name mapping is built lazily as names
are encountered and persisted in `localStorage`, so the same real name always
maps to the same fake name across toggles and page reloads. API calls continue
to use real names internally; only the displayed text is masked. A yellow
"screenshot" badge appears in the header when active. Purely client-side — no
backend changes. Keyboard hint updated to show `s` alongside existing shortcuts.

### Tests for `renderHourlyTable`
Added five test cases for the hourly breakdown table in `history_test.go`:
basic two-profile rendering, single-profile with percentage verification,
empty output for non-today entries, single-hour with 100%, and multi-hour
gap filling. A shared `mkEntry` helper builds entries dated today at a
given hour to keep tests concise.

### Dashboard: Log File Stats
The **Watch** tab now shows a compact info line at the bottom displaying log
file metadata: total entry count, file size (in human-readable KB/MB), and
the date range from oldest to newest entry. Example:
`Log: 1,234 entries · 156 KB · Feb 19 – Feb 25`. Fetched once at page load
via `/api/stats` and cached — not polled, since stats don't change fast enough
to warrant repeated requests. The line is hidden when the log is empty.

### Dashboard: Open in Browser Window (`--open`)
`notify dashboard --open` launches the dashboard in a chromeless browser window
(no address bar, no tabs) using Edge or Chrome's `--app` mode. The flag tries
Edge first, then Chrome/Chromium, and falls back to the OS default browser if
none support app mode. The browser window is a separate process — closing it
does not stop the server, and `Ctrl+C` in the terminal still shuts down the
server as usual.

### Dashboard: Silent Mode Tab
New **Silent** tab (6th tab, keyboard shortcut `6`) lets you view and control
silent mode directly from the dashboard. Shows current status (active/inactive)
with a live countdown timer, quick-set buttons for common durations (15m, 30m,
1h, 2h, 4h), a custom duration input field, and a disable button. A status
badge appears next to the tab bar on all tabs whenever silent mode is active,
showing "Silent until HH:MM:SS". Backed by `/api/silent` (GET for status, POST
to enable/disable). State changes are logged to the event log the same way the
CLI does. The dashboard polls the endpoint every 2 seconds and updates the
countdown display every second for smooth UI.

---

## 2026-02-24

### Heartbeat for Long Tasks (`--heartbeat`)
New `--heartbeat` (`-H`) flag for `notify run` fires the `"heartbeat"` action
periodically while the wrapped command runs. Useful for 30+ minute builds that
give zero feedback — a heartbeat every few minutes confirms the task hasn't
hung. `notify run --heartbeat 5m -- make build` dispatches the `"heartbeat"`
action every 5 minutes with `{command}`, `{duration}`, and `{Duration}` set to
the elapsed time. First tick fires after one interval (not immediately). If the
command finishes before the first tick, no heartbeat fires. A config-level
default (`"heartbeat_seconds"` in `"config"`) avoids needing the flag every
time — the CLI flag overrides config when both are set. Zero or omitted means
disabled. If the `"heartbeat"` action doesn't exist in the profile, an error
is printed to stderr but the wrapped command keeps running.

### Pipe / Stream Mode (`notify pipe`)
New `notify pipe [profile]` subcommand reads stdin line-by-line and triggers
notifications when patterns match. Reuses the existing `--match` infrastructure
from output capture: `tail -f build.log | notify pipe boss --match "SUCCESS"
done --match "FAIL" error`. Without `--match`, every line triggers the
`"ready"` action (useful for low-volume streams like deployment events).
First match wins when multiple patterns could match; unmatched lines are
skipped silently. The `{output}` template variable contains the matched line.
Steps with `"when": "direct"` fire in pipe mode; steps with `"when": "run"`
do not — pipe is not a command wrapper. Dispatches through the same
`dispatchActions` path as direct invocations, so cooldown, logging, echo,
silent mode, and profile auto-selection all work. Exits 0 on EOF, exits 1
on scanner error (broken pipe).

### Output Capture and Pattern Matching
`notify run` can now capture command output for use in notifications and
select actions based on output content. Set `"output_lines": N` in config
to populate the `{output}` template variable with the last N lines of
stdout/stderr. Use `--match <pattern> <action>` (repeatable) to override
exit-code-based action selection — first substring match wins, falling
back to exit code resolution if no pattern matches. Output is tee'd to
both the terminal and an internal buffer (mutex-protected for concurrent
stdout/stderr writes). Capture is only enabled when needed: either
`output_lines > 0` or `--match` flags are present, so there's zero
overhead for existing users. `{output}` is empty in non-run contexts.

### Profile Auto-Selection (Match Rules)
Profiles can now define a `"match"` object with `"dir"` and/or `"env"`
conditions. When the profile argument is omitted, `notify` checks match
rules to auto-select the right profile — no extra typing needed. `"dir"`
is a substring match against the working directory (forward-slash
normalized, so Windows backslash paths work too). `"env"` is a
`KEY=VALUE` check against an environment variable. All conditions within
a match rule are AND (both must match). If multiple profiles match, the
first alphabetically wins. Falls back to `"default"` when no rule
matches. Explicit profile (`notify boss done`) always takes priority.
Auto-selection works in direct mode, `notify run`, and `notify test`.
`notify list` shows match rules alongside aliases and extends annotations.
Config validation catches empty match rules and malformed env conditions.

### History Watch (`notify history watch`)
Live-updating dashboard that shows today's summary as a formatted table.
Clears the screen and refreshes every 2 seconds. Press `x` or `Ctrl+C`
to exit. Uses terminal raw mode for instant key detection. A "New" column
tracks per-action and per-profile deltas since watch started. The header
shows start time and elapsed duration.

Below the summary table, an hourly breakdown shows activity per hour with
one column per profile and a Total column. A `%` column shows each hour's
share of the day's total, making it easy to spot peak working hours. Rows
span from the first active hour to the current hour; quiet hours show dim
dashes so gaps in activity are visible at a glance.

### Summary Table Format
`notify history summary` and `watch` render a structured color table with
profile subtotal rows (cyan), indented per-action rows, and a bold total.
A `%` column on profile rows shows each profile's share of the grand total.
Skipped counts in yellow, new deltas in green. Profiles are separated by
blank lines. Large numbers use dot thousands separators (e.g. 1.234).
Columns (Total, Skipped, New) appear dynamically; the `%` column is always
visible. Separators and date are dim. Colors respect the `NO_COLOR`
environment variable.
`notify history summary all` shows all-time stats.

### History Clean (`notify history clean`)
New `notify history clean [days]` subcommand prunes old log entries by age.
`notify history clean 7` removes all entries older than 7 days while keeping
recent ones. `notify history clean` with no argument clears the entire log
(same as `history clear`). Prints a summary of how many entries were removed
and kept. Useful for periodic log maintenance without losing recent history.

### Per-Profile Credential Overrides
Credentials (`discord_webhook`, `slack_webhook`, `telegram_token`,
`telegram_chat_id`) can now be set per-profile in addition to globally.
Profile credentials override global ones field-by-field — set only the
fields you want to change, the rest fall through to global. This lets
different profiles post to different Discord channels, Slack channels,
or Telegram chats. Profile credentials support the same `$VAR` / `${VAR}`
environment variable expansion as global credentials. Profile inheritance
(`"extends"`) merges parent credentials into child (child wins on conflict).
Config validation uses merged credentials, so a profile with a `discord`
step only needs `discord_webhook` set somewhere — globally or on the profile.

### Surface all parallel step errors
`runner.Execute()` now returns all errors from parallel steps via
`errors.Join` instead of only the first. When multiple remote steps fail
(e.g. Discord is down *and* Telegram token expired), the user sees every
failure.

### History command tests
Added table-driven tests for `renderSummaryTable`, `aggregateGroups`,
`buildBaseline`, `fmtNum`, and `fmtPct` in `cmd/notify/history_test.go`.

### builtinSounds sync test
`TestBuiltinSoundsMatchAudio` in `config/sounds_sync_test.go` asserts
that `builtinSounds` stays in sync with `audio.Sounds` — fails if a
sound is added to or removed from either side.

---

## 2026-02-23

### Notification Groups (Comma-Separated Actions)
Fire multiple actions in a single call by separating them with commas:
`notify boss done,attention` runs `done` then `attention` in sequence.
Each action gets its own resolve, cooldown check, step filtering,
execution, logging, and echo — fully independent. If one action fails
(e.g. unknown action name), the rest still run; the process exits 1 if
any failed. Works in both direct mode and `notify run` (including
`exit_codes` mappings like `"2": "done,attention"`). Single actions
work exactly as before — no breaking changes.

### Direct Send (`notify send`)
New `notify send <type> <message>` command fires a one-off notification
without needing a profile or action defined in config. Takes the step type
and message as positional args, pulls credentials from the existing config,
and executes a single step directly. Supported types: `say`, `toast`,
`discord`, `discord_voice`, `slack`, `telegram`, `telegram_audio`,
`telegram_voice`. The `--title` flag sets a custom title for toast
notifications. `sound` and `webhook` are not supported (they need a sound
name or URL+headers, not a simple message). Template variables are expanded
in the message text. Volume is resolved from `--volume` or the config default.
Supports `--log` and `--echo` flags — log entries use `send:<type>` as the
action name (e.g. `send:telegram_voice`).

## 2026-02-22

### Profile Aliases
Profiles can now define `"aliases"` — shorthand names that resolve to the
full profile. `notify mp ready` works the same as `notify myproject ready`
when `"aliases": ["mp"]` is set on the `myproject` profile. Aliases resolve
to the canonical profile name for template variables (`{profile}` expands to
`myproject`, not `mp`) and event logging. Aliases are checked after direct
profile match but before the default fallback. `notify list` shows aliases
in the output. Config validation catches duplicate aliases (two profiles
claiming the same alias) and aliases that shadow existing profile names.
`Resolve()` now returns `(string, *Action, error)` with the canonical name.

### Template Variables: `{time}`, `{Time}`, `{date}`, `{Date}`, `{hostname}`
Five new template variables available in all text fields (`say`, `toast`,
`discord`, `slack`, `telegram`, `webhook`): `{time}` expands to compact
time (`14:30`), `{Time}` to spoken time (`2:30 PM`), `{date}` to compact
date (`2026-02-22`), `{Date}` to spoken date (`February 22, 2026`), and
`{hostname}` to the machine's hostname. Same lowercase/uppercase convention
as `{duration}`/`{Duration}` — lowercase for compact display, uppercase for
TTS. A new `baseVars()` helper in `main.go` eliminates duplication between
`runAction` and `runWrapped`.

### Config Validate Command (`notify config validate`)
New `notify config validate` command checks the config file for errors
without running any notifications. On success prints `Config OK: <path>`;
on error prints the full multi-line validation error and exits 1. Reuses
the existing `Validate()` function — zero new validation code. Also adds
`config.FindPath()` to expose config path resolution independently of
loading.

### History Export (`notify history export`)
New `notify history export [days]` subcommand outputs all log entries as a
JSON array to stdout. Each entry includes `time`, `profile`, `action`, and
`kind` (execution, cooldown, silent, other). Optional `[days]` argument
filters to recent entries. Pipe to `jq` for ad-hoc analysis. Empty logs
output `[]`.

### History Summary (`notify history summary`)
New `notify history summary [days]` subcommand parses the event log and shows
per-profile/action execution counts grouped by day (descending). Default
lookback is 7 days. Cooldown-skipped and silent-skipped invocations are shown
as a "(N skipped)" annotation alongside the execution count. `cooldown=recorded`
and `silent=enabled/disabled` entries are ignored. Also adds `notify history
clear` to delete the log file. Parser lives in `internal/eventlog/parse.go`,
co-located with the writer so format changes stay coordinated.

### Profile Inheritance (`"extends"`)
Profiles can now inherit all actions from a parent profile using
`"extends": "parent"`. The child only needs to define actions it wants to
override — all other actions are inherited from the parent. Chains are
supported (A extends B extends C) and circular chains are detected at load
time with a clear error message. Inheritance is resolved eagerly after
config load (before validation and env expansion) so all downstream code
sees fully flattened profiles. `notify list` annotates child profiles with
`(extends X)`. The `Profile` type changed from a plain map to a struct with
`Extends` and `Actions` fields, with a custom `UnmarshalJSON` that keeps
the JSON format backward-compatible — existing configs without `"extends"`
work unchanged.

### Exit Code Mapping
`notify run` previously hardcoded exit 0 → `ready` and non-zero → `error`.
New `"exit_codes"` map in the `"config"` section lets users route specific
exit codes to custom actions, e.g. `"2": "warning"` or `"130": "cancelled"`.
Keys are exit code strings, values are action names. Unmapped codes still
use the default 0→ready / non-zero→error fallback. Config validation checks
that keys parse as integers and values are non-empty. Config is now loaded
before exit code resolution in `runWrapped` to make the mapping available.

### Custom Sound Files
The `sound` step now accepts a WAV file path alongside the 7 built-in sound
names. If the value isn't a recognized built-in name, it's treated as a file
path. Relative paths are resolved against the config file's directory, so
`"pling.wav"` finds the file next to `notify-config.json`. Supports PCM WAV
files with 8-bit, 16-bit, or 24-bit sample depth, mono or stereo, at any
sample rate. Files are automatically decoded and resampled to 44100 Hz stereo
16-bit for playback through the same audio pipeline as built-in sounds.
Compressed WAV formats (A-law, mu-law, ADPCM, etc.) are rejected with a
clear error. `notify play` also accepts WAV file paths for previewing.

### Generic Webhook Step
New `webhook` step type — HTTP POST to an arbitrary URL with the message as
body. Covers ntfy.sh, Pushover, Home Assistant, IFTTT, or any custom endpoint.
URL and optional headers live on the step itself (not in credentials), so one
config can target multiple endpoints. Headers support `$VAR` / `${VAR}`
expansion for secrets via `os.ExpandEnv`. Default `Content-Type` is
`text/plain`; custom headers can override it. Body uses template variable
expansion like other step types. Requires `url` and `text` fields. Runs in
parallel with automatic retry on transient errors.

### Retry for Remote Steps
Remote notification steps (`discord`, `discord_voice`, `slack`, `telegram`,
`telegram_audio`, `telegram_voice`) now automatically retry once after a
2-second delay if the network call fails. Only the HTTP send is retried —
local prep work like TTS rendering, temp file creation, and ffmpeg conversion
is not repeated. No configuration needed; the retry is always active. This
makes notifications resilient to transient network errors without adding
complexity to the config.

### Environment Variables in Credentials
Credential values (`discord_webhook`, `slack_webhook`, `telegram_token`,
`telegram_chat_id`) now support `$VAR` and `${VAR}` syntax to reference
environment variables. Expansion happens at config load time using Go's
`os.ExpandEnv`, so all downstream code sees resolved values transparently.
Undefined variables resolve to empty strings, which the existing config
validation catches as missing credentials. Literal URLs pass through
unchanged. This makes it safe to version-control configs or share them
across machines without exposing secrets.

### Sound Preview (`notify play`)
New `notify play [sound]` command lets you audition built-in sounds without
creating a config action. With no arguments, lists all 7 available sounds
with descriptions. With a sound name, plays it immediately using the CLI
`--volume` flag or full volume by default. Unknown sound names show an error
with the list of valid names.

### Notification History (`notify history`)
New `notify history [N]` command pretty-prints the last N entries from
`notify.log` (default 10). Reads the log file location via the existing
`eventlog.LogPath()` function. If the log file doesn't exist, prints a
friendly message suggesting `--log` or `"log": true`. Useful for
reviewing recent notification activity without opening the log file
manually.

### Echo Option (`--echo`, `-E`)
New `--echo` (`-E`) CLI flag and `"echo": true` config option prints a
one-line summary of the step types that ran after each invocation, e.g.
`notify: sound, say, toast`. If all steps were filtered out:
`notify: no steps ran`. Same opt-in pattern as `--log` and `--cooldown`
(config bool + CLI flag). Useful for confirming what fired without
enabling full event logging.

### `"when": "never"` Condition
New `"when": "never"` condition that always skips the step. Add it to
temporarily disable a step without removing it from config; remove it
to re-enable.

### Telegram Voice Bubbles (`telegram_voice`)
New `telegram_voice` step type generates TTS audio, converts WAV to OGG/OPUS
via `ffmpeg`, and uploads to Telegram via the Bot API `sendVoice` endpoint.
Renders as a native voice bubble in Telegram clients (unlike `telegram_audio`
which displays as an inline audio player). Uses platform-native TTS engines
(`SayToFile`) to render speech to a temp file, then `ffmpeg -c:a libopus` for
the format conversion. Requires `ffmpeg` on PATH, `credentials.telegram_token`
and `credentials.telegram_chat_id` in config. Same `when` filtering and
template variables as other Telegram steps.

### Slack Webhook Notifications (`slack`)
New `slack` step type posts messages to a Slack channel via incoming webhook.
Uses a simple JSON POST (`{"text": "..."}`) — same pattern as the existing
`discord` step. Requires `credentials.slack_webhook` in config. Supports
template variables and `when` filtering. Runs in parallel (no audio pipeline
dependency).

### Cooldown Auto-Pruning
`record()` now prunes expired entries from `cooldown.json` on every write.
Entries older than 24 hours or with unparseable timestamps are deleted
before the new entry is written, preventing unbounded file growth from
long-running watch loops.

### Silent Mode (`notify silent`)
New `notify silent <duration>` command temporarily suppresses all
notification execution without editing the config. Supports Go-style
duration strings (`30s`, `5m`, `1h`, `2h30m`). `notify silent off`
disables immediately; `notify silent` (no args) shows current status.
During silent mode, both direct invocations and `notify run` exit
immediately — no sound, no speech, no toast, no remote notifications.
Suppressed invocations are still logged when event logging is enabled,
as are enable/disable state changes.
`notify test` shows silent status in its output. State is stored in
`silent.json` in the notify data directory, with fail-open semantics
(missing, corrupt, or expired files are treated as not silent).

### Telegram Audio Messages (`telegram_audio`)
New `telegram_audio` step type generates TTS audio and uploads it to Telegram
as a WAV file via the Bot API `sendAudio` endpoint. Uses platform-native TTS
engines (`SayToFile`) to render speech to a temp file, then uploads via
multipart POST with the text as a caption. Same `when` filtering and template
variables as the existing `telegram` step. Requires `credentials.telegram_token`
and `credentials.telegram_chat_id` in config. Displays as an inline audio
player in Telegram.

### Toast Validation
Config validation now checks that `toast` steps have a `message` field set.
Previously only `sound`, `say`, `discord`, and `telegram` steps had
required-field validation.

### API Error Body in Messages
Discord and Telegram API error messages now include up to 200 bytes of the
response body alongside the HTTP status code. Previously only the status
code was reported, making it hard to debug issues like invalid tokens or
rate limiting.

### Test Coverage
Added unit tests for `audio` (PCM generation, volume scaling, sound
registry completeness) and `eventlog` (step detail formatting for all
step types, template expansion, default toast title). Added missing
config validation tests for `discord_voice` and `telegram_audio`
credential and required-field checks, plus `log` config parsing tests.

### AppleScript Escaping Extraction
Moved inline `escapeAppleScript()` from `toast_darwin.go` to
`shell/escape_darwin.go` as `EscapeAppleScript()`, consistent with the
existing `EscapePowerShell()` pattern in `shell/escape.go`.

### Runner Credential Check Cleanup
Removed redundant credential validation from `runner.go` — config
validation already catches missing credentials at load time. The runner
checks were defensive duplicates that could never trigger after a
successful `Validate()` call.

### Help Output
`notify help` and `notify -h` now show a link to the GitHub documentation
at the top of the output.

## 2026-02-21

### Discord Voice Messages (`discord_voice`)
New `discord_voice` step type generates TTS audio and uploads it to Discord
as a WAV file attachment. Uses platform-native TTS engines (`SayToFile`)
to render speech to a temp file, then uploads via multipart POST with the
text as a caption. Same `when` filtering and template variables as the
existing `discord` step. Requires `credentials.discord_webhook` in config.

### Dry-Run Command (`notify test`)
New `notify test [profile]` command loads config, validates it, detects
AFK state, then shows what would happen for every action in the profile
without actually firing notifications. Each step is marked RUN or SKIP
based on current `when` conditions. Useful for debugging new configs.

### Credential Completeness Validation
Config validation now checks that `discord` steps have
`credentials.discord_webhook` set, and `telegram` steps have both
`credentials.telegram_token` and `credentials.telegram_chat_id` set.
Catches missing credentials at config load time instead of at execution.

### `detectAFK` Testability
The idle detection function is now a package-level variable, allowing
unit tests to inject mock idle times. Three new tests cover AFK, present,
and error (fail-open) scenarios.

### Telegram Notifications
New `telegram` step type sends messages via the Telegram Bot API.
Requires `telegram_token` and `telegram_chat_id` in the config
credentials block. Uses form-encoded POST to the `sendMessage` endpoint.
Same template variables and `when` filtering as the existing `discord`
step type. The `Execute` function now takes a `Credentials` struct
instead of a single webhook URL, making it straightforward to add
future remote notification platforms.

### Config Validation
New `Validate()` function in the config package catches common mistakes
at load time instead of failing silently at runtime. Checks: unknown
step types, invalid `when` conditions (including malformed `hours:X-Y`
specs), missing required fields per step type (`sound` for sound steps,
`text` for say/discord), volume ranges (0-100) on both per-step and
global default, negative cooldown/AFK thresholds, and empty actions.
Collects all problems into a single multi-line error. Called
automatically after config load in all code paths.

### Path Resolution Consolidation
Extracted shared `internal/paths` package consolidating triplicated
platform-specific path resolution logic from config, cooldown, and
eventlog into a single `DataDir()` function. Shared constants for file
names and permissions replace scattered magic numbers. Extracted
`resolveVolume`, `resolveCooldown`, `detectAFK`, and `shouldLog` helpers
in `main.go` to eliminate duplication between `runAction` and `runWrapped`.

### Cooldown / Rate Limiting
Per-action cooldown prevents notification spam from watch loops and file
watchers. Cooldown is opt-in: enable with `--cooldown` (`-C`) on the CLI
or `"cooldown": true` in the config `"config"` block. Set a global default
duration with `"cooldown_seconds"` in config, or override per-action.
When enabled, if the same profile+action was triggered within the cooldown
window, the invocation exits silently. Cooldown state is stored in
`cooldown.json` in the notify data directory. Skipped and recorded cooldown
events are logged when event logging is enabled.

### Log File Location
The event log (`notify.log`) and cooldown state (`cooldown.json`) now
live in the notify data directory (`%APPDATA%\notify\` on Windows,
`~/.config/notify/` on Linux/macOS) instead of the home directory.

## 2026-02-20

### Opt-in Event Logging
Event logging is now opt-in instead of always writing a log on every
invocation. Enable with `--log` (`-L`) on the CLI or `"log": true` in
the config `"config"` block. Without either, no log file is written.

### Quiet Hours (`hours:X-Y`)
New time-based `"when"` condition suppresses steps outside a given hour
range. `"hours:8-22"` runs the step only between 8am and 10pm;
`"hours:22-8"` handles cross-midnight ranges. Invalid specs are
fail-closed (step skipped with stderr warning). Pairs naturally with
AFK detection — suppress sound at night while keeping toast/discord
active, all within one profile.

### Command Wrapper (`notify run`)
New `notify run [profile] -- <command...>` subcommand wraps an arbitrary
command, captures its exit code and duration, then automatically triggers
the `ready` action on success (exit 0) or `error` on failure. Three new
template variables: `{command}` (the wrapped command), `{duration}`
(compact: `2m15s`), and `{Duration}` (spoken: `2 minutes and 15 seconds`
— ideal for TTS). The `--` separator is required to distinguish notify
flags from the wrapped command. The wrapped command's exit code is
preserved as notify's own exit code.

New `"when"` conditions `"run"` and `"direct"` let steps target a
specific invocation mode — use `"when": "run"` for steps that only
make sense with command context (like speaking the duration), and
`"when": "direct"` for steps that should only run in normal invocations.

### CI and Collaboration
Added CI workflow that runs `go vet`, tests, and cross-platform build
checks on every push and PR to `main`. Added `CONTRIBUTING.md` with
setup instructions and PR guidelines. Added issue templates for bug
reports and feature requests.

### Version Command
`notify version` (also `-V`, `--version`) prints version, build date,
and platform. Version and build date are injected via `-ldflags` in CI;
local builds show `dev`. Help output now shows the version in its
header and author credit with links at the bottom.

### GitHub Actions Release Workflow
Added automated release pipeline triggered by version tags (`v*`).
Builds binaries for all 5 platform targets (linux/amd64, linux/arm64,
windows/amd64, darwin/amd64, darwin/arm64), runs tests first, then
creates a GitHub Release with all binaries, `THIRD_PARTY_LICENSES`,
and the example config attached. Linux builds use `CGO_ENABLED=1`
with ALSA headers (oto requires CGO on Linux); Windows and macOS
builds use `CGO_ENABLED=0` (oto uses purego on these platforms).

### Config Structure Reorganization
Config file now uses explicit `"config"` and `"profiles"` top-level
sections instead of a flat layout. Global options (AFK threshold,
default volume, credentials) live under `"config"`, profile definitions
live under `"profiles"`. This also simplified the custom UnmarshalJSON
from raw-map plucking to a standard type-alias approach with defaults.

### Default Volume in Config
New `"default_volume"` key in the `"config"` section (0-100) sets the
baseline volume when `--volume` is not passed on the command line.
Per-step `volume` overrides still take highest priority. Defaults to
100 if omitted.

### Discord Webhook Notifications
New `discord` step type posts messages to a Discord channel via webhook.
Credentials are stored in `"config"` → `"credentials"`. Discord steps
run in parallel (no audio pipeline dependency) and
support template variables (`{profile}`, `{Profile}`). Pairs naturally
with AFK detection — use `"when": "afk"` to send a Discord message when
you're away from your machine.

### AFK Detection
Per-step `"when"` condition flag (`"afk"`, `"present"`, or omitted for
always-run) lets a single action handle both cases — e.g. play a sound
when present, show a toast when away. Configurable idle threshold via
`"afk_threshold_seconds"` in notify-config.json (default 300s).
Platform-native idle detection: Windows `GetLastInputInfo`, macOS
`ioreg` HIDIdleTime, Linux `xprintidle`. Fails open on error.

### Event Log Improvements
Log now records only steps that actually executed (AFK-filtered steps
are omitted). Blank line separates each invocation for readability.

### Code Quality
Extracted duplicated `expand`/`titleCase` helpers into `internal/tmpl`
and `escapePowerShell` into `internal/shell`. Added unit tests for
config parsing, template expansion, PowerShell escaping, and AFK step
filtering.

### Template Variables
Added `{profile}` and `{Profile}` (title-cased) placeholders for use
in `say` text, `toast` title/message, and `discord` text. Useful with the default
profile fallback — one action definition produces different messages
depending on which profile was invoked.

### Config File Rename
Renamed config from generic `config.json` to `notify-config.json` to
avoid conflicts with other tools.

### Event Log
Added append-only invocation log (`notify.log` in the notify data
directory) with summary and per-step detail lines. Template variables
are expanded in log output. Best-effort — errors print to stderr but
never fail the command.

### Default Profile
Profile argument is now optional; omitting it defaults to `"default"`.

## 2026-02-19

### Initial Release
`notify` CLI tool that runs multi-step notification pipelines from a
JSON config file. Each action can combine sound, speech, and toast
steps. Supports multiple profiles with automatic fallback to `"default"`
profile. Config file resolution: explicit `--config` path, next to
binary, or user config directory. Built with Go and
[oto](https://github.com/ebitengine/oto) for cross-platform audio
(Windows WASAPI, macOS Core Audio, Linux ALSA). CMake build system with
cross-compilation targets. Volume control via `--volume` flag (0-100).
