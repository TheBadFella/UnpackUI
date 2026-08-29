# Environment variables

All configuration environment variables use the `UN_` prefix. UnpackUI
supports the complete upstream Unpackerr variable set and adds the variables
below.

## Added by UnpackUI

| Variable | Default | What it does |
|---|---:|---|
| `UN_SUPPRESS_MISSING_URLS` | `true` | Silently skips optional Starr entries with no URL. Missing API keys, invalid configured URLs, and connection failures are still logged. |
| `UN_STATE_FILE` | automatic | Stores watched-folder recovery state so interrupted work can be re-queued after restart. Use a writable persistent path, or `off` to disable recovery. |
| `UN_WEB_URL` | empty | Sets the externally reachable status UI URL used by native Discord links. It does not enable the web server by itself. |
| `UN_WEBSERVER_API` | `false` | Enables the aggregate, path-free JSON endpoint at `/api/stats`. The UI also makes this endpoint available when `UN_WEBSERVER_UI=true`. |
| `UN_WEBSERVER_UI` | `false` | Enables the status dashboard, its detailed status API, adaptive polling, and clear-completed action. |
| `UN_WEBHOOK_<n>_UPDATE_EXISTING` | `false` | For native Discord hooks, creates one message per extraction and edits it as status changes. Replace `<n>` with the zero-based webhook index. |

These are fork additions relative to upstream Unpackerr at the UnpackUI v1.7.0
sync point. Settings such as `UN_REMNANT_ACTION` are inherited from upstream,
even if they first appear in this release.

## Related upstream variables

The following inherited variables are commonly used with the fork features:

| Variable | What it does |
|---|---|
| `UN_WEBSERVER_LISTEN_ADDR` | Address and port for the web server; defaults to `0.0.0.0:5656`. |
| `UN_WEBSERVER_URLBASE` | Serves the UI and API below a path prefix such as `/unpackui`. |
| `UN_WEBSERVER_METRICS` | Enables the Prometheus `/metrics` endpoint independently of the UI. |
| `UN_WEBSERVER_SSL_CERT_FILE` / `UN_WEBSERVER_SSL_KEY_FILE` | Enables direct HTTPS when both files are set. |
| `UN_WEBSERVER_UPSTREAMS` | Comma-separated trusted reverse-proxy IPs or CIDRs for forwarded client addresses. |
| `UN_WEBHOOK_<n>_URL` | Destination URL for an indexed webhook. |
| `UN_WEBHOOK_<n>_TEMPLATE` | Forces a built-in template such as `discord` or `notifiarr`; otherwise the URL is auto-detected. |
| `UN_WEBHOOK_<n>_EVENTS_<m>` | Selects event IDs for a webhook. Use `UN_WEBHOOK_0_EVENTS_0=0` for all events. |
| `UN_WEBHOOK_<n>_EXCLUDE_<m>` | Excludes an application such as `lidarr` or `folder` from a webhook. |
| `UN_REMNANT_ACTION` | Controls destination files left by interrupted extractions (`rename`, `delete`, or `off`; default `rename`). This applies to Starr jobs and watched folders with `move_back=true`. |
| `UN_MAX_BYTES` | Caps uncompressed bytes written per archive; defaults to `75GB`. Use `0` for unlimited. |
| `UN_MAX_FILES` | Caps files, directories, and symlinks created per archive; defaults to `5000`. Use `0` for unlimited. |
| `UN_MAX_RATIO` | Caps the extraction expansion ratio; defaults to `15`. Use `0` for unlimited. |

## Indexing examples

```yaml
environment:
  # First webhook, all events.
  UN_WEBHOOK_0_URL: https://discord.com/api/webhooks/replace/me
  UN_WEBHOOK_0_TEMPLATE: discord
  UN_WEBHOOK_0_UPDATE_EXISTING: "true"
  UN_WEBHOOK_0_EVENTS_0: "0"

  # Second webhook.
  UN_WEBHOOK_1_URL: https://notifiarr.com/api/v1/notification/unpackerr/key
  UN_WEBHOOK_1_TEMPLATE: notifiarr

  # First Sonarr instance and its first path.
  UN_SONARR_0_URL: http://sonarr:8989
  UN_SONARR_0_API_KEY: replace-with-your-api-key
  UN_SONARR_0_PATHS_0: /downloads
```

For every inherited global, Starr, folder, webhook, and command-hook variable,
see the exhaustive [generated Compose reference](../examples/docker-compose.yml)
or the [upstream documentation](https://unpackerr.zip).
