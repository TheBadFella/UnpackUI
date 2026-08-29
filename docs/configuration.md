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

# Per-archive disk protection. Use 0 to disable an individual limit.
max_bytes = "75GB"
max_files = 5000
max_ratio = 15

# Persist watched-folder work so interrupted items can be retried after restart.
state_file = "/config/unpackerr.state.json"

# Handle extracted destination files left by an interrupted extraction.
remnant_action = "rename"

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
  UN_MAX_BYTES: 75GB
  UN_MAX_FILES: "5000"
  UN_MAX_RATIO: "15"
  UN_STATE_FILE: /config/unpackerr.state.json
  UN_REMNANT_ACTION: rename
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
against the configured watch folders, cleans known partial sidecar output for
queued or extracting items, and re-queues valid work from the beginning. It
does not resume an archive at the exact byte where extraction stopped, and it
does not persist Starr queue items.

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

## Interrupted-extraction remnants

`remnant_action` handles a different problem from restart recovery. If an
extracted file cannot be moved to its final destination because that path
already exists, Unpackerr compares the blocker with a snapshot taken before the
extraction. Files that were already part of the download are protected; files
created afterward can be treated as output left by an interrupted extraction.

This check applies to Starr jobs and watched folders with `move_back = true`.
The default watched-folder mode extracts into a sidecar directory and does not
classify destination remnants this way.

- `rename` (default) moves each blocker to a sibling ending in `.remnant`
  (or a numbered variant), rolls back newly moved sibling output, and retries.
- `delete` removes each blocker, rolls back newly moved sibling output, and
  retries.
- `off` leaves blockers untouched and fails without retrying that item.

Handling is bounded by `max_retries`. This is a per-file destination-conflict
safety mechanism; it does not replace the watched-folder `state_file` recovery
described above.

## Extraction safety limits

The upstream protection merged from PR #667 applies three per-archive limits:

- `max_bytes` / `UN_MAX_BYTES` caps uncompressed bytes written. Size strings
  such as `75GB` and `100GiB` are accepted.
- `max_files` / `UN_MAX_FILES` caps files, directories, and symlinks created.
- `max_ratio` / `UN_MAX_RATIO` caps bytes written divided by archive size. A
  value of `15` allows a 1 GB archive to write at most 15 GB.

The defaults are 75 GB, 5,000 entries, and 15:1. Set an individual value to `0`
to make that limit unlimited. Raise limits deliberately for trusted archives
whose legitimate contents exceed these defaults.

## Complete references

- [Fork-specific environment variables](environment-variables.md)
- [Generated example config](../examples/unpackerr.conf.example)
- [Generated Compose environment reference](../examples/docker-compose.yml)
- [Upstream Unpackerr documentation](https://unpackerr.zip)

The examples are generated from `init/config/definitions.yml` and provide the
exhaustive reference for inherited upstream options in this repository.
