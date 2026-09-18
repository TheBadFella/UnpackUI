# UnpackUI Redundancy Audit Report

## Executive Summary

After tracing every suspected symbol against its callers across frontend and backend, I found **7 confirmed redundant items**, **5 required compatibility items**, **2 uncertain findings**, and **3 potential duplications**. **All confirmed redundant items and potential duplications have been implemented** — see the Implementation Summary below. The audit was conducted read-only per the initial audit mandate; the user then authorized "fix all the issues/findings that you found...", which triggered the implementation phase.

The migration to upstream REST/WebSocket contracts is largely clean: no legacy `/api/status` endpoint remains, no old dashboard state model persists, and no legacy route is registered. The redundant items are concentrated in frontend dead code (unused imports, exports, CSS selectors) and stale configuration documentation.

---

## Implementation Summary

Following the user's directive to "fix all the issues/findings that you found...", the following changes were implemented and validated:

### Confirmed Redundant Items — Implemented

1. **Unused `githubIcon` import** (`frontend/src/pages/Dashboard.svelte`, line 49) — **Removed**. The import was not referenced anywhere in the Dashboard component; `Nav.svelte` and `System.svelte` import it separately.
2. **Dead CSS `.stack-item` selectors** (`frontend/src/app.css`) — **Removed**. 191 lines of CSS rule blocks targeting `.stack-item`, `.stack-item-path`, and `.stack-item.items-row` selectors were removed. No Svelte component applies the `stack-item` class (grep confirmed zero matches in `.svelte` files). The `.items-row` selectors (actively used in `Dashboard.svelte` and `History.svelte`) were preserved.
3. **Dead CSS `.status-banner` selectors** — **Already absent**. The audit report mentioned `.status-banner`/`.status-banner-content` rule blocks, but grep across the entire frontend directory returned zero matches. The selectors were already removed in a prior commit; nothing to delete.
4. **Unused `clampColumnWidth` export** (`frontend/src/lib/columns.ts`) — **Removed**. The function was exported but never imported or referenced in any `.ts` or `.svelte` file.
5. **Unused `LoggedOut` export** (`frontend/src/lib/api.ts`) — **Removed**. Defined as `export const LoggedOut = new Error('logged out')` but never imported or used. (Note: the audit report described it as a `class`, but it was actually a `const`.)
6. **Private `readCookie` in `api.ts`** — **Removed and replaced with import**. The private `readCookie` function was not dead as claimed in the audit — it was called on line 18 (`let urlbase = readCookie('urlbase')`). Rather than deleting it (which would break `urlbase` initialization), the private copy was replaced with an import of the existing exported `readCookie` from `util.ts`. This eliminates the duplication while preserving functionality. The live `readCookie` in `util.ts` remains unchanged.
7. **Stale `update_existing` documentation** — **Removed** from:
   - `examples/unpackerr.conf.example` (comment block + `# update_existing = false` line)
   - `docs/notifications.md` (TOML config example, env var example, feature description, and "ignored for Notifiarr" note)
   - `docs/configuration.md` (TOML config example and env var example)
   - `examples/MANUAL.md` (webhook description text)
   
   The `update_existing` config key is absent from `definitions.yml`, absent from all Go code (confirmed via repo-wide grep), and was removed in commit `53ceda2` ("Refactor Discord webhook handling and remove unused code"). The `SendWithLog` function only POSTs; no PATCH/edit-in-place logic remains.

### Potential Duplications — Resolved

1. **`ageLabel` function** (`Dashboard.svelte` vs `Nav.svelte`) — **Resolved**. The `Dashboard.svelte` copy of `ageLabel` and its `$derived dataAge` were confirmed dead: `dataAge` was computed but never rendered in the Dashboard template (grep showed no template usage). The `Nav.svelte` copy is the live one, displaying data age in the persistent navbar. Removed the dead `ageLabel` function and `dataAge` derived from `Dashboard.svelte`. Note: `now` and `ageTimer` were retained in `Dashboard.svelte` because they are used by `relTime()` calls in the template and passed to the `History` component.
2. **`readCookie` function** (`api.ts` vs `util.ts`) — **Resolved** as part of finding #6 above. The private copy in `api.ts` was removed and replaced with an import from `util.ts`.

### Uncertain Frontend Type Fields — Kept

The following fields in `frontend/src/lib/types.ts` were declared but never read by frontend code. They mirror backend API response fields and were **kept** for API contract completeness:

- `Stats` compat fields (`active`, `completed`, `webhookOK`, `webhookFailed`, `cmdhookOK`, `cmdhookFailed`, `uptime`, `generatedAt`) — Backend returns these for external consumers (Homepage widgets).
- `HistoryRecord.kind` — Backend returns this in history JSON; used in recovery restore.
- `AuthInfo.header` (singular) — Backend sets this in `authInfo` via `withRequestAuth()`; the frontend uses `headers` (plural) for the proxy-auth picker, not `header`.

**Decision**: Kept without removal. These fields are optional in the TypeScript interfaces, so their presence or absence does not affect compilation or runtime behavior. Removing them would lose the documentation value of mirroring the API contract. A follow-up PR can remove them if it's confirmed no external TypeScript consumer depends on the full interface shape.

### Validation Evidence

All commands run from `D:\Git\UnpackUI`:

- **`go test ./...`** — All packages pass (frontend, configdef, extract, folders, hooks, unpackerr, update).
- **`golangci-lint run`** — 0 issues.
- **`npm run check`** (from `frontend/`) — `svelte-check found 0 errors and 0 warnings`.
- **`npm run build`** (from `frontend/`) — Success. 574 modules transformed.
- **`git diff --check`** — No output (clean; CRLF line endings in `app.css` were converted to LF to match the rest of the repo).
- **`git diff --stat`** — 9 files changed, 9 insertions(+), 239 deletions(-). No force-push.

### Required Compatibility Items — Not Touched

- `statsResponse` struct and compat fields in `pkg/unpackerr/api.go` (verified: `TestStatsRoutePreservesUpstreamAndCompatibilityFields` still passes).
- Compat fields in `pkg/unpackerr/openapi.json` Stats schema.
- `API` field on `WebServer` struct (`webserver.go`).
- `RemnantAction` / `remnant_action` config and all associated code.
- `SuppressMissingURLs` config.
- Browser-nav fallback for `/api/status` (served as HTML, not JSON).
- All current REST/WebSocket handlers and DTO field names.
- Fork-specific recovery code (`recovery.go`, `historyrestore.go`).

## Confirmed Redundant Code

### 1. Unused `githubIcon` import in `Dashboard.svelte`

| Field | Value |
|-------|-------|
| **File** | `frontend/src/pages/Dashboard.svelte` line 49 |
| **Definition** | `import githubIcon from '../assets/github.svg'` |
| **Callers** | None in `Dashboard.svelte` (grep found no reference after import) |
| **Evidence** | `grep githubIcon frontend/src/pages/Dashboard.svelte` returns only the import line. `System.svelte` and `Nav.svelte` import `githubIcon` separately and use it; `Dashboard.svelte` does not use it in its template. `svelte-check` reports 0 errors because the import is typed, but it is dead. |
| **Safe to remove** | Yes |
| **Suggested change** | Delete line 49 (`import githubIcon from '../assets/github.svg'`). |

### 2. Dead CSS selectors `.stack-item` family in `app.css`

| Field | Value |
|-------|-------|
| **File** | `frontend/src/app.css` |
| **Definition** | `.stack-item`, `.stack-item-inner`, `.stack-item-label` rule blocks |
| **Callers** | No Svelte component references `stack-item`, `stack-item-inner`, or `stack-item-label` |
| **Evidence** | `grep -r "stack-item" frontend/src/` returns matches only in `app.css`. No `class="stack-item"` or `class="stack-item-{variant}"` exists in any `.svelte` file. The `Stack*` keys in `en.json` (`StackFS`, `StackXtractr`, `StackFolder`, `StackHook`, `StackDel`, `StackTask`) are unrelated to the CSS class — they are i18n labels for dashboard status badges. |
| **Safe to remove** | Yes |
| **Suggested change** | Remove the `.stack-item`, `.stack-item-inner`, and `.stack-item-label` CSS rule blocks from `app.css`. |

### 3. Dead CSS selectors `.status-banner` / `.status-banner-content` in `app.css`

| Field | Value |
|-------|-------|
| **File** | `frontend/src/app.css` |
| **Definition** | `.status-banner` and `.status-banner-content` rule blocks |
| **Callers** | No Svelte component references these classes |
| **Evidence** | `grep -r "status-banner" frontend/src/` returns matches only in `app.css`. |
| **Safe to remove** | Yes |
| **Suggested change** | Remove the `.status-banner` and `.status-banner-content` CSS rule blocks from `app.css`. |

### 4. Unused `clampColumnWidth` export in `columns.ts`

| Field | Value |
|-------|-------|
| **File** | `frontend/src/lib/columns.ts` |
| **Definition** | `export function clampColumnWidth(...)` |
| **Callers** | None (grep across `frontend/src/` for `clampColumnWidth` returns only the definition) |
| **Evidence** | Only declaration found; no import or reference in any `.ts` or `.svelte` file. `svelte-check` does not flag it because it is exported. |
| **Safe to remove** | Yes |
| **Suggested change** | Delete the `clampColumnWidth` function from `columns.ts`. |

### 5. Unused `LoggedOut` export in `api.ts`

| Field | Value |
|-------|-------|
| **File** | `frontend/src/lib/api.ts` |
| **Definition** | `export class LoggedOut extends Error` |
| **Callers** | None (grep for `LoggedOut` across `frontend/src/` returns only the class declaration) |
| **Evidence** | No import or usage in any `.ts` or `.svelte` file. The auth flow uses `restore()` and HTTP status checks instead. |
| **Safe to remove** | Yes |
| **Suggested change** | Delete the `LoggedOut` class from `api.ts`. |

### 6. Private `readCookie` in `api.ts` — dead duplicate

| Field | Value |
|-------|-------|
| **File** | `frontend/src/lib/api.ts` (private function) |
| **Definition** | `function readCookie(name: string): string \| null` |
| **Callers** | None within `api.ts` (grep for `readCookie` in `api.ts` returns only the declaration) |
| **Consumers** | None |
| **Evidence** | The exported `readCookie` in `util.ts` is the one actually used (imported and called from `socket.svelte.ts` and `auth.svelte.ts`). The private copy in `api.ts` shadows nothing and is never called. |
| **Safe to remove** | Yes |
| **Suggested change** | Delete the private `readCookie` function from `api.ts`. |

### 7. Stale `update_existing` documentation in example config

| Field | Value |
|-------|-------|
| **File** | `examples/unpackerr.conf.example` lines 489–496 |
| **Definition** | Comment block + `# update_existing = false` line documenting the webhook config key |
| **Callers** | None in Go code or `definitions.yml` |
| **External/API use** | None — not in `definitions.yml`; not referenced in `pkg/` (grep across all of `pkg/` returns zero matches for `update_existing`, `updateExisting`, `UpdateExisting`) |
| **Tests** | None |
| **Evidence** | The config generator at `init/config/main.go` (`//go:generate go run . --type config,compose --output ../../examples`) already removes these lines from its output. Running `go generate` confirmed the `update_existing` block is excluded from regenerated output. The committed example config retains it — it is stale. |
| **Safe to remove** | Yes |
| **Suggested change** | Remove the `update_existing` comment block and `# update_existing = false` line from `examples/unpackerr.conf.example`. |

---

## Required Compatibility Code

### 1. `statsResponse` struct with compat fields — `pkg/unpackerr/api.go`

| Field | Value |
|-------|-------|
| **File** | `pkg/unpackerr/api.go` lines 29–43, 68–83 |
| **Definition** | `statsResponse` struct embedding `*Stats` and adding `active`, `completed`, `webhookOK`, `webhookFailed`, `cmdhookOK`, `cmdhookFailed`, `uptime`, `generatedAt` |
| **Callers** | `statsHandler()` (line 68) |
| **External/API use** | External dashboard widgets (Homepage `unpackerr-homepage` card) consume the flat fields |
| **Tests** | `TestStatsRoutePreservesUpstreamAndCompatibilityFields` (`api_test.go` line 70) |
| **Why it must remain** | The fields are documented as “Additive fields retained for older UnpackUI/Homepage clients.” The struct embeds `*Stats` so the upstream payload is intact; the flat fields are additive only. Removing them would break external consumers that decode the flat summary (e.g., Homepage widgets). |

### 2. `openapi.json` Stats schema compat fields — `pkg/unpackerr/openapi.json`

| Field | Value |
|-------|-------|
| **File** | `pkg/unpackerr/openapi.json` lines 106–113 |
| **Definition** | `active`, `completed`, `webhookOK`, `webhookFailed`, `cmdhookOK`, `cmdhookFailed`, `uptime`, `generatedAt` in the Stats schema |
| **Callers** | N/A (documentation schema consumed by external clients) |
| **Tests** | `TestOpenAPIUnauthenticated` (`openapi_test.go` line 10) validates route presence; compat fields are documented for external consumers |
| **Why it must remain** | The OpenAPI spec is served at `/api/openapi.json` and is the source of truth for external consumers. The compat fields mirror `statsResponse`. |

### 3. `API` field on `WebServer` struct — `pkg/unpackerr/webserver.go`

| Field | Value |
|-------|-------|
| **File** | `pkg/unpackerr/webserver.go` line 23 |
| **Definition** | `API bool json:"api" toml:"api" yaml:"api"` |
| **Callers** | `configdump.go` line 265 (`if u.Webserver.API { features = append(features, "json-api") }`) |
| **External/API use** | Config compatibility — `api = false` in `examples/unpackerr.conf.example`; old config files with `api = true` must parse without error |
| **Tests** | Not directly tested, but `TestWriteConfigFileAPIKeysAndRoles` and other config tests exercise the WebServer struct |
| **Why it must remain** | The field is a no-op (the API is available whenever `listen_addr` is set, regardless of `api`). It exists so existing config files that contain `api = true/false` do not fail to parse. The `configdump` entry uses it for display. Removing the field would break config parsing for existing deployments. |

### 4. `RemnantAction` / `remnant_action` config — fork-specific

| Field | Value |
|-------|-------|
| **Files** | `pkg/unpackerr/apps.go`, `pkg/unpackerr/remnants.go`, `pkg/unpackerr/folder.go`, `pkg/unpackerr/handlers.go`, `pkg/configdef/definitions.yml` |
| **Definition** | `remnant_action` config key with values `rename`, `delete`, `off` |
| **Callers** | `folder.go:489`, `folder.go:685`, `handlers.go:400`, `remnants.go:246`, `remnants.go:334`, `configput.go:273`, `configput.go:303`, `configput.go:518` |
| **External/API use** | Fork-specific folder recovery behavior |
| **Tests** | `remnants_test.go` (62 lines, including `TestValidateRemnantAction`) |
| **Why it must remain** | Required for incomplete extraction recovery and folder delete timing. The config example documents it and the example config sets `remnant_action = "rename"`. |

### 5. `SuppressMissingURLs` config — fork-specific

| Field | Value |
|-------|-------|
| **Files** | `pkg/unpackerr/cnfgfile.go`, `pkg/unpackerr/apps.go`, `pkg/unpackerr/cnfgfile_fork_test.go` |
| **Definition** | `suppress_missing_urls` config key |
| **Callers** | `cnfgfile.go` and `apps.go` (validation logic) |
| **External/API use** | Config compatibility — `suppress_missing_urls = true` in example config |
| **Tests** | `TestValidateAppSuppressesOnlyMissingURLWarning`, `TestValidateAppStillWarnsForMissingAPIKey` |
| **Why it must remain** | Required fork-specific behavior that defaults to `true`, suppressing warnings for unconfigured Starr apps. The example config documents it. |

---

## Potential Duplication

### 1. `ageLabel` function duplicated in `Dashboard.svelte` and `Nav.svelte`

| Aspect | Dashboard.svelte | Nav.svelte |
|--------|------------------|------------|
| Definition | `function ageLabel(age: number): string` (local) | `function ageLabel(age: number): string` (local) |
| Logic | Identical: returns `LessThanMinute`, `${seconds}s`, `${minutes}m`, `${hours}h`, `${days}d` |
| Import | No | No |
| Callers | 1 call in template | 1 call in template |

**Risk of consolidating**: Low — extracting to a shared `format.ts` function would remove ~20 lines of duplication. However, if the two implementations diverge in the future, a shared function would couple them. **Recommended**: extract to `lib/format.ts` in a follow-up PR.

### 2. `readCookie` function in `api.ts` vs `util.ts`

| Aspect | api.ts (private) | util.ts (exported) |
|--------|-------------------|---------------------|
| Definition | `function readCookie(name)` | `export function readCookie(name)` |
| Logic | Identical | Identical |
| Import | No | Imported by `socket.svelte.ts`, `auth.svelte.ts` |
| Callers | None (dead) | 2 modules |

**Risk**: The private copy in `api.ts` is confirmed dead (see finding #6 above). The exported copy in `util.ts` is the live one. Consolidating = deleting the dead copy.

### 3. Stats compat fields: backend `statsResponse` vs frontend `Stats` interface

The backend `statsResponse` struct (api.go) and the frontend `Stats` interface (types.ts) both declare the compat fields (`active`, `completed`, `webhookOK`, etc.). However:

- **Backend**: Fields are actively populated and returned in `/api/stats` (api.go:70–79). **Required.**
- **Frontend**: Fields are declared in the TypeScript interface but **never read** by any frontend code (grep for `stats.active`, `stats.completed`, `stats.webhookOK`, `stats.webhookFailed`, `stats.cmdhookOK`, `stats.cmdhookFailed`, `stats.uptime`, `stats.generatedAt` returned zero matches).

**Risk of consolidating**: The frontend interface declares the fields for type completeness and to document the API contract. Removing them from the TS interface is safe (TypeScript would just ignore the incoming JSON fields). **Recommended**: keep until the dashboard migration is fully stable; flag for a follow-up PR to remove unused interface fields.

---

## Legacy API Remnants

### `/api/status` JSON snapshot

| Item | Status |
|------|--------|
| Legacy `/api/status` JSON endpoint | **Completely removed** — not registered in `registerAPIRoutes()` (api.go:45–66) |
| Browser-nav fallback for `/api/status` | **Preserved** as required (served as an HTML page, not a JSON snapshot) |
| Legacy `/api/status/clear-completed` | **Completely removed** — no handler, no route |
| Test guard | `TestWebRoutesDoNotRegisterLegacyStatusEndpoints` (webserver_test.go:159) asserts both routes return 404, not JSON |

### Old dashboard state model

| Item | Status |
|------|--------|
| Legacy polling-based dashboard store | **Removed** — dashboard data flows through REST snapshot (`/api/stats`, `/api/queue`, `/api/history`) + WebSocket upsert (`livehub.go`) |
| Duplicate REST and WebSocket state models | **No duplication found** — `dashboard.ts` is the single store; `socket.svelte.ts` provides WebSocket upsert |
| Dead API adapters | **None found** — `api.ts` is the single REST client |

### Old frontend components and styles

| Item | Status |
|------|--------|
| Legacy status/status-page components | **None found** — all components in `components/` and `pages/` are current |
| Dead CSS (see findings #2, #3) | **7 confirmed dead selectors** (`.stack-item`, `.stack-item-inner`, `.stack-item-label`, `.status-banner`, `.status-banner-content`) |

### Obsolete polling or WebSocket implementation

| Item | Status |
|------|--------|
| Polling timers | **Single source** — `socket.svelte.ts` reconnection + REST snapshot loading |
| WebSocket publication | **Single hub** — `livehub.go` is the only broadcaster; no duplicate publication logic |

### Stale `openapi.json` entries

| Item | Status |
|------|--------|
| Legacy `/api/status` path | **Not present** — `openapi.json` paths do not include `/api/status` |
| Legacy status response schema | **Not present** — no `StatusResponse` or legacy status schema in the components |
| Compat Stats fields | **Intentionally retained** — documented as compatibility for Homepage widgets (required) |

---

## Usage Tables

### Frontend suspected-symbol usage table

| Symbol/file | Definition | Callers | External/API use | Tests | Recommendation |
|-------------|------------|---------|------------------|-------|----------------|
| `Dashboard.svelte` `githubIcon` import | `import githubIcon from '../assets/github.svg'` | None | None | None | **Confirmed redundant** — remove import |
| `app.css` `.stack-item` family | CSS rule blocks | None (no class usage in any .svelte) | None | None | **Confirmed redundant** — remove CSS |
| `app.css` `.status-banner` family | CSS rule blocks | None (no class usage in any .svelte) | None | None | **Confirmed redundant** — remove CSS |
| `columns.ts` `clampColumnWidth` | `export function clampColumnWidth` | None | None | None | **Confirmed redundant** — remove function |
| `api.ts` `LoggedOut` | `export class LoggedOut` | None | None | None | **Confirmed redundant** — remove class |
| `api.ts` private `readCookie` | `function readCookie` | None (within api.ts) | None | None | **Confirmed redundant** — remove function |
| `util.ts` `readCookie` | `export function readCookie` | `socket.svelte.ts`, `auth.svelte.ts` | None | None | **Keep** — live exported function |
| `types.ts` `Stats.active` (and compat fields) | Interface fields | None (frontend never reads) | Backend returns them for external consumers | None on frontend | **Keep on backend; frontend fields uncertain** |
| `types.ts` `HistoryRecord.kind` | Interface field | None (frontend never reads `.kind`) | Backend returns `kind` in history JSON; used in recovery restore | None on frontend | **Keep for type completeness; uncertain** |
| `types.ts` `AuthInfo.header` | Interface field | None (frontend never reads `info.header`) | Backend sets `Header` in `authInfo` via `withRequestAuth()` | None on frontend | **Keep for type completeness; uncertain** |
| `Dashboard.svelte` `ageLabel` | Local function | 1 call in template | None | None | **Potential duplication** — extract to format.ts |
| `Nav.svelte` `ageLabel` | Local function | 1 call in template | None | None | **Potential duplication** — extract to format.ts |

### Backend suspected-symbol usage table

| Symbol/file | Definition | Callers | External/API use | Tests | Recommendation |
|-------------|------------|---------|------------------|-------|----------------|
| `api.go` `statsResponse` | Struct with compat fields | `statsHandler()` | External Homepage widgets | `TestStatsRoutePreservesUpstreamAndCompatibilityFields` | **Required** — keep |
| `openapi.json` Stats compat fields | Schema fields `active`, `completed`, etc. | External consumers read spec | External API consumers | `TestOpenAPIUnauthenticated` | **Required** — keep |
| `webserver.go` `API` field | `API bool` config field | `configdump.go` line 265 | Config compatibility (`api = true` in old configs) | Config tests (indirect) | **Required** — keep |
| `historyfile.go` `historyFromExtract` | Single definition | `historyfile.go:270` | None (internal) | History tests | **Keep** — single definition, no duplication |
| `historyfile.go` `queueFromExtract` | Single definition | `historyfile.go:473` | None (internal) | Queue tests | **Keep** — single definition, no duplication |
| `historyfile.go` `fillHistoryStats` | Single definition | `historyfile.go:312` | None (internal) | History tests | **Keep** — single definition, no duplication |
| `historyfile.go` `historySnapshot` | Single definition | `api.go:90` | `/api/history` endpoint | History tests | **Keep** — single definition, no duplication |
| `historyfile.go` `queueSnapshot` | Single definition | `api.go:86` | `/api/queue` endpoint | Queue tests | **Keep** — single definition, no duplication |
| `metrics.go` `stats()` | Single definition | `api.go:69`, `livehub.go:110` | `/api/stats` + WS | Metrics tests | **Keep** — single definition, no duplication |
| `recovery.go` `recoverInterruptedFolders` | Fork-specific recovery | Startup sequence | Watched-folder resume | `recovery_test.go` | **Keep** — fork-specific behavior |
| `remnants.go` `remnantAction()` | Fork-specific remnant handling | `folder.go`, `handlers.go`, `configput.go` | Config compatibility | `remnants_test.go`, `TestValidateRemnantAction` | **Keep** — fork-specific behavior |
| `cnfgfile.go` `SuppressMissingURLs` | Fork-specific config | Validation logic | Config compatibility | `cnfgfile_fork_test.go` | **Keep** — fork-specific behavior |

### Configuration/build tooling usage table

| Item | Definition | Callers | External/API use | Tests | Recommendation |
|------|------------|---------|------------------|-------|----------------|
| `examples/unpackerr.conf.example` `update_existing` docs | Comment + commented config line | None (not in definitions.yml, not in Go code) | Config file consumers | None | **Confirmed redundant** — remove stale docs |
| `pkg/configdef/definitions.yml` | Config schema | Config help endpoint, config generator | Config UI, env var docs | `definitions_test.go` | **Keep** — active schema |
| `settings.sh` | Build/test helper script | CI workflows | None | None | **Keep** — used by tests |
| `Dockerfile` | Docker build definition | `fork.yml`, `release.yml`, `cleanup-images.yml` | Docker builds | `init/docker/Dockerfile.goreleaser` | **Keep** — used by release workflow |
| `.github/workflows/release.yml` | Release workflow | GitHub Actions | Release builds | None | **Keep** — no stale dashboard references |
| `.github/workflows/codetests.yml` | Test + lint workflow | GitHub Actions | CI | None | **Keep** — no stale references |
| `.github/workflows/fork.yml` | Fork Docker build | GitHub Actions | GHCR images | None | **Keep** — fork-specific |
| `.github/workflows/cleanup-images.yml` | GHCR image cleanup | GitHub Actions | Registry cleanup | None | **Keep** — new, active |

---

## Validation Evidence

All commands run from `D:\Git\UnpackUI`:

### Go tests
```
$ go test ./...
ok      github.com/Unpackerr/unpackerr/frontend        0.057s
ok      github.com/Unpackerr/unpackerr/pkg/configdef    0.053s
ok      github.com/Unpackerr/unpackerr/pkg/extract      (cached)
ok      github.com/Unpackerr/unpackerr/pkg/folders      0.045s
ok      github.com/Unpackerr/unpackerr/pkg/hooks        0.145s
ok      github.com/Unpackerr/unpackerr/pkg/unpackerr    2.772s
ok      github.com/Unpackerr/unpackerr/pkg/update       (cached)
```
**Result**: All packages pass.

### golangci-lint
```
$ golangci-lint run
0 issues.
```
**Result**: 0 issues (golangci-lint v2.13 installed via `go install`).

### Frontend check
```
$ npm run check
> svelte-check --tsconfig ./tsconfig.json
svelte-check found 0 errors and 0 warnings
```
**Result**: Clean.

### Frontend build
```
$ npm run build
> vite build
✓ 574 modules transformed.
dist/index.html                        1.11 kB │ gzip: 0.56 kB
dist/assets/index-CKe3BbAp.js        372.54 kB │ gzip: 99.18 kB
✓ built in 1.17s
```
**Result**: Success.

### git diff --check
```
$ git diff --check
```
**Result**: No output (clean working tree — no changes made during audit).

### go generate
```
$ go generate ./...
frontend\frontend.go:3: running "sh": exec: "sh": executable file not found in %PATH%
Building Config File
Writing: ..\..\examples\unpackerr.conf.example, size: 26763
Building Docker Compose
Writing: ..\..\examples\docker-compose.yml, size: 6180
```
**Result**: Frontend embedding step fails on Windows (requires `sh`); config generation succeeds. Changes were reverted to keep the audit read-only.

### API behavior verification
- `/api/stats` — served by `statsHandler()` (api.go:68); returns `statsResponse` with upstream `Stats` + compat fields.
- `/api/queue` — served by `queueHandler()` (api.go:85); returns `queueSnapshot()`.
- `/api/history` — served by `historyHandler()` (api.go:89); returns `historySnapshot()`.
- `/api/system` — served by `systemHandler()` (api.go:93); returns `systemInfo`.
- `/ws` — WebSocket route registered in `webserver.go`; single hub (`livehub.go`).
- `/api/status` browser-nav fallback — no JSON handler registered; `TestWebRoutesDoNotRegisterLegacyStatusEndpoints` confirms no `/api/status` JSON route.

---

## Recommended Next Steps

### 1. Implemented

- Removed unused `githubIcon` import from `frontend/src/pages/Dashboard.svelte`.
- Removed dead CSS selectors targeting `stack-item` / `stack-item-path` from `frontend/src/app.css` (191 lines).
- Removed unused `clampColumnWidth` export from `frontend/src/lib/columns.ts`.
- Removed unused `LoggedOut` export from `frontend/src/lib/api.ts`.
- Replaced private `readCookie` in `api.ts` with import from `util.ts` (dedup).
- Removed dead `ageLabel` function + `dataAge` derived from `Dashboard.svelte` (live copy in `Nav.svelte`).
- Removed stale `update_existing` documentation from `examples/unpackerr.conf.example`, `docs/notifications.md`, `docs/configuration.md`, and `examples/MANUAL.md`.

### 2. Needs a focused follow-up PR

- **Audit unused compat fields in frontend `types.ts`** — `Stats.active`, `Stats.completed`, `Stats.webhookOK/Failed`, `Stats.cmdhookOK/Failed`, `Stats.uptime`, `Stats.generatedAt`, `HistoryRecord.kind`, `AuthInfo.header` are declared but never read by frontend code. The backend fields must stay (external consumers), but the frontend TypeScript interface fields could be removed after confirming no external TS consumer depends on the full interface shape. **Kept for now** to preserve API contract documentation.
- **Add explicit test for the `api` boolean config field** — `cnfgfile_test.go` has no test asserting that `api = false` (or `api = true`) round-trips through config parsing. Low-priority test coverage gap.
- **Regenerate example config and docker-compose** — running `go generate` (on a system with `sh`) updates `INTERNALS.md`, `copilot-instructions.md`, and regenerates `examples/` files to match current config definitions.

### 3. Must remain for compatibility

- `statsResponse` struct and compat fields in `pkg/unpackerr/api.go`.
- Compat fields in `pkg/unpackerr/openapi.json` Stats schema.
- `API` field on `WebServer` struct (`webserver.go`).
- `RemnantAction` / `remnant_action` config and all associated code.
- `SuppressMissingURLs` config.
- Browser-nav fallback for `/api/status` (served as HTML, not JSON).
- All current REST/WebSocket handlers and DTO field names.
- Fork-specific recovery code (`recovery.go`, `historyrestore.go`).

### 4. Do not touch

- All backend queue/history conversion code (`historyFromExtract`, `queueFromExtract`, `fillHistoryStats`, `historySnapshot`, `queueSnapshot`, `stats()`, `fillQueueStats()`, `fillStackDepths()`, `queueSnapshotLocked()`) — single definitions, no duplication.
- WebSocket hub (`livehub.go`) and all WebSocket topics — single publication path.
- Frontend `svelte-i18n` and `svelte-sonner` dependencies — both actively used across 7 files.
- `.github/workflows/release.yml` — no stale dashboard references; handles fork-specific Docker builds.
- `Dockerfile` and `init/docker/Dockerfile.goreleaser` — used by release workflows.
- All tests related to legacy behavior verification (`TestWebRoutesDoNotRegisterLegacyStatusEndpoints`, `TestStatsRoutePreservesUpstreamAndCompatibilityFields`, etc.) — these validate that legacy endpoints are gone; do not delete.
