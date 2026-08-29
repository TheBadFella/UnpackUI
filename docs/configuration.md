# Configuration

UnpackUI accepts the same TOML configuration and `UN_` environment variables
as upstream Unpackerr. This fork adds a small set of settings for recovery, the
status UI/API, and Discord notification updates.

## Precedence

UnpackUI loads a config file first, then applies environment variables. An
environment variable therefore overrides the corresponding file value.

Common config-file locations are:

- Docker and Linux: `/config/unpackerr.conf`, `/etc/unpackerr/unpackerr.conf`,
  `~/.unpackerr/unpackerr.conf`, or `./unpackerr.conf`
- Windows: `C:\ProgramData\unpackerr\unpackerr.conf`,
  `~\.unpackerr\unpackerr.conf`, or `.\unpackerr.conf`
- macOS: `~/.unpackerr/unpackerr.conf`,
  `/usr/local/etc/unpackerr/unpackerr.conf`, or `/etc/unpackerr/unpackerr.conf`

Pass `-c /path/to/unpackerr.conf` to select a file explicitly.

## Fork feature example

```toml
# Ignore empty optional Starr entries without hiding real configuration errors.
suppress_missing_urls = true

# Persist watched-folder work so interrupted items can be retried after restart.
state_file = "/config/unpackerr.state.json"

# Publicly reachable UI URL used in native Discord notification links.
web_url = "https://unpackui.example.com"

[webserver]
ui = true
api = true
metrics = false
listen_addr = "0.0.0.0:5656"
urlbase = "/"

[[webhook]]
name = "Discord"
url = "https://discord.com/api/webhooks/replace/me"
template = "discord"
update_existing = true
events = [0]
```

The equivalent environment variables are:

```yaml
environment:
  UN_SUPPRESS_MISSING_URLS: "true"
  UN_STATE_FILE: /config/unpackerr.state.json
  UN_WEB_URL: https://unpackui.example.com
  UN_WEBSERVER_UI: "true"
  UN_WEBSERVER_API: "true"
  UN_WEBSERVER_METRICS: "false"
  UN_WEBSERVER_LISTEN_ADDR: 0.0.0.0:5656
  UN_WEBSERVER_URLBASE: /
  UN_WEBHOOK_0_NAME: Discord
  UN_WEBHOOK_0_URL: https://discord.com/api/webhooks/replace/me
  UN_WEBHOOK_0_TEMPLATE: discord
  UN_WEBHOOK_0_UPDATE_EXISTING: "true"
  UN_WEBHOOK_0_EVENTS_0: "0"
```

List settings such as webhooks and Starr applications are zero-indexed in
environment variables. A second webhook begins with `UN_WEBHOOK_1_`; a second
Sonarr instance begins with `UN_SONARR_1_`.

## Recovery state

`state_file` stores a small JSON record of watched-folder items that were
waiting, queued, or extracting. On restart, UnpackUI validates those entries
against the configured watch folders and retries valid interrupted work.

When no explicit path is set, UnpackUI uses `/config/unpackerr.state.json` if
`/config` exists, otherwise a file beside `log_file`, otherwise a file beneath
`~/.unpackerr`. Set `state_file = "off"` or `UN_STATE_FILE=off` to disable it.

Keep the state file on persistent storage and writable by the container user.

## Optional Starr entries

`suppress_missing_urls = true` is the default. Empty Starr entries, including
the generated Sonarr and Radarr placeholders, are skipped without an error log.
Set it to `false` if you want missing-URL warnings while troubleshooting.

This option does not hide problems for an app that has a URL. Missing API keys,
invalid URLs, failed requests, and other runtime errors are still logged.

## Complete references

- [Fork-specific environment variables](environment-variables.md)
- [Generated example config](../examples/unpackerr.conf.example)
- [Generated Compose environment reference](../examples/docker-compose.yml)
- [Upstream Unpackerr documentation](https://unpackerr.zip)

The generated examples are the source of truth for inherited upstream options.
