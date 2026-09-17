# Dashboard UI and API

UnpackUI adds a built-in dashboard to Unpackerr's web server. It does not need a
separate frontend service or database.

## Enable the UI

With environment variables:

```yaml
ports:
  - "5656:5656"
environment:
  UN_WEBSERVER_UI: "true"
  UN_WEBSERVER_LISTEN_ADDR: 0.0.0.0:5656
```

With TOML:

```toml
[webserver]
ui = true
listen_addr = "0.0.0.0:5656"
```

Open `http://localhost:5656`. If `urlbase` is set to `/unpackui`, open
`http://localhost:5656/unpackui/` instead.

The first startup generates a UI password and an administrator API key when
they are not configured. Read the startup log, sign in as `admin`, and store
the generated values securely. Browser password login uses Web Crypto, so use
HTTPS or open the service through `localhost`.

## Dashboard behavior

The dashboard combines the upstream API resources with the live WebSocket:

- `/api/stats` supplies counters, queue-buffer depth, and Starr activity-queue
  summaries.
- `/api/queue` supplies active extraction records, including progress, speed,
  ETA, output paths, and retry/forget actions.
- `/api/history` supplies durable completed records, extracted-file details, and
  history delete/clear actions.
- `/ws` sends queue, progress, history, log, and error updates. The dashboard
  takes a REST snapshot after login and whenever the socket reconnects.

The UI derives active/completed counts and relative times in the browser. It
uses the upstream status labels and keeps marker files named
`_unpackerred*.txt` out of extracted-file details. Queue and history tables are
responsive; desktop column widths are saved per browser and can be reset.

## HTTP endpoints

| Endpoint | Requires | Purpose |
|---|---|---|
| `/` | `ui = true` | Dashboard and browser sign-in page. |
| `/api/stats` | `read:system:stats` | Upstream counters, buffer capacities, Starr queue summaries, and additive compatibility aliases. |
| `/api/queue` | `read:system:queue` | Current extraction queue. |
| `/api/queue/retry` | `write:system:queue` | Retry a failed queue item. |
| `/api/queue/forget` | `write:system:queue` | Forget a terminal queue item. |
| `/api/history` | `read:system:history` | Persisted extraction history. |
| `/api/history/delete` | `write:system:history` | Delete one persisted history row. |
| `/api/history/clear` | `write:system:history` | Delete all persisted history rows after confirmation. |
| `/api/system` | `read:system:info` | Version, uptime, bind address, URL base, and runtime information. |
| `/ws` | matching topic permission | Live queue, progress, history, log, and error frames. |
| `/api/config/{section}` | matching config permission | Read or update one configuration section. |
| `/api/openapi.json` | none | OpenAPI 3 description for the upstream API. |

Every route is placed below `urlbase` except the upstream metrics compatibility
route. For example, `/api/stats` becomes `/unpackui/api/stats` when
`urlbase = "/unpackui"`.

## Homepage widget

The aggregate API works with Homepage's `customapi` widget:

```yaml
- Media:
    - UnpackUI:
        icon: unpackerr.png
        href: http://unpackui:5656
        widget:
          type: customapi
          url: http://unpackui:5656/api/stats
          headers:
            X-Api-Key: replace-with-a-read-only-api-key
          mappings:
            - field: extracted
              label: Extracted
              format: number
            - field: deleted
              label: Deleted
              format: number
            - field: waiting
              label: Waiting
              format: number
            - field: extracting
              label: Extracting
              format: number
            - field: failed
              label: Failed
              format: number
```

The response also provides `queued`, `imported`, `active`, `completed`,
`finished`, `retries`, webhook and command-hook counters, `uptime`, and
`generatedAt`. The flat `active`, `completed`, `webhookOK`,
`webhookFailed`, `cmdhookOK`, and `cmdhookFailed` fields are additive
compatibility aliases; the upstream `hookOK`, `hookFail`, `cmdOK`, and
`cmdFail` fields remain authoritative.

## Network security

The web server starts whenever `listen_addr` is set. Its API uses named keys,
roles, and permissions. Send a key in `X-Api-Key` or as an
`Authorization: Bearer` token. Browser sessions use `ui_password`; it may also
be set to `webauth:<Header>` behind a trusted proxy or to `noauth` on a trusted
network. Keep the queue, history, WebSocket, and configuration endpoints off
untrusted networks even when authentication is enabled.

`api` and `UN_WEBSERVER_API` remain accepted for compatibility with older
UnpackUI configurations, but no longer gate the upstream API. Disable the HTTP
server by setting `listen_addr = ""`.

When using a reverse proxy, set `upstreams` (or `UN_WEBSERVER_UPSTREAMS`) to the
proxy IP/CIDR. This controls trusted forwarded client addresses and `webauth`
headers; do not trust a network that can be reached directly by clients.
