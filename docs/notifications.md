# Notifications

UnpackUI retains upstream webhook support for Notifiarr, Discord, Telegram,
Slack, Gotify, Pushover, and custom templates. This fork adds richer native
Discord embeds and links back to the status UI.

## Native Discord

Create an Incoming Webhook in Discord, then add:

```toml
web_url = "https://unpackui.example.com"

[[webhook]]
name = "Discord"
url = "https://discord.com/api/webhooks/replace/me"
template = "discord"
events = [0]
timeout = "10s"
```

Or use environment variables:

```yaml
environment:
  UN_WEB_URL: https://unpackui.example.com
  UN_WEBHOOK_0_NAME: Discord
  UN_WEBHOOK_0_URL: https://discord.com/api/webhooks/replace/me
  UN_WEBHOOK_0_TEMPLATE: discord
  UN_WEBHOOK_0_EVENTS_0: "0"
```

`template = "discord"` is optional for a normal Discord webhook URL because it
can be detected automatically, but setting it makes the intent explicit.

With `template = "discord"`, UnpackUI posts one message for each extraction
event. Discord message IDs are stored only in memory. If UnpackUI restarts
during an extraction, the next event creates a new message.

`web_url` should be the URL a notification reader can actually reach. It adds an
**Open UI** link and makes the native Discord title link to the dashboard. It
does not enable or expose the UI; configure the web server separately.

## Event selection

Use event `0` for all statuses, or select individual IDs:

| ID | Event |
|---:|---|
| `0` | All events |
| `1` | Queued |
| `2` | Extracting |
| `3` | Extraction failed |
| `4` | Extracted |
| `5` | Imported |
| `6` | Deleting |
| `7` | Delete failed |
| `8` | Deleted |
| `9` | Nothing extracted |

In TOML, use `events = [1, 3, 4, 8]`. With environment variables, use one
indexed variable per value:

```yaml
UN_WEBHOOK_0_EVENTS_0: "1"
UN_WEBHOOK_0_EVENTS_1: "3"
UN_WEBHOOK_0_EVENTS_2: "4"
UN_WEBHOOK_0_EVENTS_3: "8"
```

## Notifiarr

Notifiarr remains supported:

```toml
[[webhook]]
name = "Notifiarr"
url = "https://notifiarr.com/api/v1/notification/unpackerr/replace-with-key"
template = "notifiarr"
events = [0]
```

Set global `web_url` if you want the URL included in the Notifiarr payload.

## Multiple destinations and exclusions

Repeat `[[webhook]]` blocks for multiple destinations. With environment
variables, increment the webhook index: `UN_WEBHOOK_0_*`, `UN_WEBHOOK_1_*`, and
so on.

Use `exclude = ["lidarr", "folder"]` in TOML, or indexed variables such as
`UN_WEBHOOK_0_EXCLUDE_0=lidarr`, to suppress notifications from selected
applications.

For every inherited webhook field and custom-template support, see the
[generated config reference](../examples/unpackerr.conf.example) and the
[upstream documentation](https://unpackerr.zip).
