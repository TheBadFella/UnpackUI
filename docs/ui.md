# Status UI and API

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

The dashboard shows aggregate counters and current/recent extraction items. It
includes status, application, progress, ETA, retries, elapsed time, output
details, and delete countdowns when available.

- The dashboard polls every 2 seconds while work is active and every 30 seconds
  while idle. A failed request is retried after 10 seconds.
- Drag a table header divider to resize a column. Widths are stored in that
  browser's local storage; **Reset columns** restores the defaults.
- **Clear completed** removes completed rows from the in-memory UI history. It
  does not delete downloaded or extracted files.
- On narrow screens, the table becomes a mobile-friendly card layout.

## HTTP endpoints

| Endpoint | Requires | Purpose |
|---|---|---|
| `/` | `ui = true` | Dashboard and browser sign-in page. |
| `/api/status` | `read:system:queue` | Detailed dashboard state. This may include paths and extraction details. |
| `/api/status/clear-completed` | `write:system:history` | `POST` action that hides completed rows from the dashboard. |
| `/api/stats` | `read:system:stats` | Flat aggregate counters without download paths or Starr details. |
| `/api/queue` | `read:system:queue` | Current extraction queue. |
| `/api/history` | `read:system:history` | Persisted extraction history. |
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
`generatedAt`.

## Network security

The web server starts whenever `listen_addr` is set. Its API uses named keys,
roles, and permissions. Send a key in `X-Api-Key` or as an
`Authorization: Bearer` token. Browser sessions use `ui_password`; it may also
be set to `webauth:<Header>` behind a trusted proxy or to `noauth` on a trusted
network. Keep the detailed status, queue, history, and configuration endpoints
off untrusted networks even when authentication is enabled.

`api` and `UN_WEBSERVER_API` remain accepted for compatibility with older
UnpackUI configurations, but no longer gate the upstream API. Disable the HTTP
server by setting `listen_addr = ""`.

When using a reverse proxy, set `upstreams` (or `UN_WEBSERVER_UPSTREAMS`) to the
proxy IP/CIDR. This controls trusted forwarded client addresses and `webauth`
headers; do not trust a network that can be reached directly by clients.
