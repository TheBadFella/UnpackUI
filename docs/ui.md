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
  UN_WEBSERVER_API: "true"
  UN_WEBSERVER_LISTEN_ADDR: 0.0.0.0:5656
```

With TOML:

```toml
[webserver]
ui = true
api = true
listen_addr = "0.0.0.0:5656"
```

Open `http://localhost:5656`. If `urlbase` is set to `/unpackui`, open
`http://localhost:5656/unpackui/` instead.

## Dashboard behavior

The dashboard shows aggregate counters and current/recent extraction items. It
includes status, application, progress, ETA, retries, elapsed time, output
details, and delete countdowns when available.

- Updates arrive live over a WebSocket, with polling as a fallback.
- Drag a table header divider to resize a column. Widths are stored in that
  browser's local storage; **Reset columns** restores the defaults.
- **Clear completed** removes completed rows from the in-memory UI history. It
  does not delete downloaded or extracted files.
- On narrow screens, the table becomes a mobile-friendly card layout.

## HTTP endpoints

| Endpoint | Requires | Purpose |
|---|---|---|
| `/` | `ui = true` | Dashboard page. |
| `/api/status` | `ui = true` | Detailed dashboard state. This may include paths and extraction details. |
| `/api/status/clear-completed` | `ui = true` | `POST` action used by the dashboard to clear completed history. |
| `/ws` | `ui = true` | Live dashboard updates. |
| `/api/stats` | `ui = true` or `api = true` | Flat aggregate counters without download paths or Starr details. |

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

The built-in web server does not provide authentication. Keep it on a trusted
network or place it behind an authenticated reverse proxy before exposing it to
the internet. The detailed UI API can contain filesystem paths; use only
`api = true` without `ui = true` when an external dashboard needs aggregate
counts but should not receive item details.

When using a reverse proxy, set `upstreams` (or `UN_WEBSERVER_UPSTREAMS`) to the
proxy IP/CIDR so forwarded client addresses are trusted correctly.
