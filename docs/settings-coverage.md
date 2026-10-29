# Settings coverage

Reviewed on 2026-10-29 against the backend configuration, configuration API,
and all settings pages, including the shared Starr interval controls and
notification profile fields. The additions below first shipped in v2.0.16.
Implementation: [3ccfe3a](https://github.com/TheBadFella/UnpackUI/commit/3ccfe3a).

## Missing controls found

| Setting | In v2.0.15 | Available from v2.0.16 |
|---|---|---|
| Watched-folder `disable_log` | Accepted by the API but has no control | **Folders → Extraction → Create extraction log**. Disabled writes `disable_log = true`; restart to apply. |
| Global `log_file_mode` | Accepted by the API but has no control | **General → Logging → Log file permissions**, on POSIX hosts. Restart to apply to new log files. |
| Global `timeout` | Accepted by the API but has no control | **General → Extraction → Default request timeout**. Restart to rebuild existing clients and hooks. |

The audit also found that global `timeout` and `delete_delay` were absent from
the generated-file schema. UI saves omitted both values. Their configured
values are now written to TOML and covered by a save/reload regression test.
This fixes value persistence; saves still regenerate the complete file.

## Settings that remain outside the UI

| Setting | How to configure it | Reason or limit |
|---|---|---|
| `state_file` | Global TOML or `UN_STATE_FILE` | Recovery storage is resolved at startup. Stop the service before changing the location. Use persistent storage; `off` disables recovery. |
| `web_url` | Global TOML or `UN_WEB_URL` | Public UI URL used by notification links. It is not included in the current General settings API. Restart after changing it. |
| `suppress_missing_urls` | Global TOML or `UN_SUPPRESS_MISSING_URLS` | Startup validation behavior for optional Starr entries; not included in the General settings API. Defaults to `true`. Restart after changing it. |
| `webserver.api`, `webserver.ui` | Accepted from existing files/environment | Compatibility inputs. They no longer enable or disable routes. Use `webserver.listen_addr` to control the server. |
| Global `[folders].interval` / `UN_FOLDERS_INTERVAL` | Existing file/environment configurations | Compatibility fallback. Use each watched folder's **Poll interval** for new configurations. An explicit per-folder value wins. |
| Singular Starr `path` | Existing file/API/environment configurations | Compatibility input. Use **Download paths** (`paths`) for current configurations. |

Command-line inputs such as `-c` select startup behavior rather than editable
configuration fields. Runtime queue data, generated authentication state, and
internal clients and counters are not settings controls.

## Where the remaining settings appear

| Backend settings | UI location and conditions |
|---|---|
| `debug`, `quiet`, `error_stderr`, `log_file`, `log_files`, `log_file_mb`, `progress` | **General → Logging**. `error_stderr` is ignored on Windows. |
| `parallel`, `max_retries`, `start_delay`, `retry_delay`, `delete_delay`, `keep_history`, `passwords`, `remnant_action` | **General → Extraction**. |
| `file_mode`, `dir_mode` | **General → Extraction**, on POSIX hosts. |
| `interval`, `log_queues`, `activity` | Shared controls above the **Starr** application list. These were already visible in v2.0.15. |
| Server address, URL base, metrics, profiling, TLS certificate/key and access-log settings | **Webserver** settings. |
| UI password, proxy role header, trusted upstreams, WebSocket origins, API keys and roles | **Webserver** authentication and permissions controls. Some fields appear only for the selected authentication mode. |
| Starr name, URL, API key, basic/native credentials, download paths, protocols, timeout, cleanup delay, archive deletion, Syncthing, SSL validation and max bytes | Each **Starr** application's form. Environment-owned fields remain locked. |
| `split_flac`, `ape_format`, `ape_compression` | **Lidarr** form; compression controls follow the selected format. |
| Folder event buffer | **Folders**, above the watched-folder list. |
| Watch/extract paths, poll interval, cleanup, move-back, ISO extraction, executable bits, empty-folder filtering, recursion, exclusions, wait extensions, symlinks and archive limits | Each **Folders** entry. `disable_log` completes the folder setting controls in this release. |
| Notification `custom_ids` and event `titles` | **Notifications → Payload**. |
| Webhook URL/name, events, exclusions, nickname, channel, token, templates, updates, SSL handling, timeout, content type and headers | **Notifications → Webhooks**. Fields depend on the selected notification profile; advanced fields can be expanded. Built-in profiles supply their own content type. |
| Command, name, shell, silent output, events, exclusions and timeout | **Notifications → Command hooks**. HTTP-only webhook fields do not apply. |

Permissions can make a page read-only or hide it entirely. Environment-owned
fields show the running value while preserving the file value separately.
See [configuration precedence and saving](configuration.md#saving-from-the-ui)
for the effect on the config file.
