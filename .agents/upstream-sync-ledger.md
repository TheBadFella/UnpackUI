# Upstream sync ledger

This ledger records the last reviewed `upstream/main` commit so future syncs can
inspect only the upstream commits that arrived after that point. It is a review
baseline, not a replacement for the repository contracts in
[`repository-context.md`](repository-context.md).

## Baseline recorded 2026-10-29

- Last reviewed upstream commit: `afa377da4d13c736f8bcbbbf1756e9a6817deeee`
  (PR #838, Vite 8.3.5). Previous baseline: `3ff4271eadae0c294e354458a86ca5f15be05e37`.
- Reviewed 26 upstream commits across 30 paths (653 insertions, 270 deletions).
  Merged in `f1cadcd`; final dependency reconciliation in `821f4fb`.
- Adopted plain-HTTP UUID generation, blank login username, one-minute queue-log
  default, Starr path-list controls and indexed/comma-separated environment
  paths, editable file-owned fields on environment-configured instances,
  notification profile/credential display fixes, Node 26 CI, Vite 8.3.5,
  svelte-check 4.7.7, Carbon 2.7.0 and bytefmt 0.95.0.
- Preserved singular Starr `path` as a compatibility input and help/API alias.
  The frontend alias is optional; new forms use `paths`. Retained fork release
  workflows, explicit disabled folder cleanup (`delete_after = "0s"`), default
  missing-URL suppression, recovery, recursive folders, confinement and webhook
  deduplication.
- Dependabot PRs #66–#69 merged with passing checks. #67's original integration
  failure was a smoke-test port collision (`bind: address already in use`);
  its final head `f655c22` passed all six checks before merge.
- Aligned the root source-build Dockerfile with #69's Alpine digest; the bot
  had only changed the GoReleaser Dockerfile. Registry inspection confirmed
  the new index and supported architectures. No user configuration changes.
- This release removes `.commandcode/taste` and light-mode selection, adds
  watched-folder extraction-log, global request-timeout and POSIX log-mode
  controls, and improves the shared task dialog and keyboard focus handling.
- The settings audit found global `timeout` and `delete_delay` missing from the
  TOML renderer schema. Added both and verified non-default save/reload values,
  extraction-log suppression and log permissions. Config saves still regenerate
  the whole file and keep one `.bak`; preserving custom formatting is deferred.
- Local verification: `go test ./...`, native Windows `go generate ./...` using
  Git sh, cross-target `GOOS=linux golangci-lint run ./...` (0 issues), frontend
  check (0 errors/warnings), 53 Vitest tests in 17 files, 7 detail-data tests,
  production frontend build, and diff checks passed.
- Browser review used synthetic API data at 320, 390, 768 and 1440 pixels:
  queue/history dialogs, long paths, empty/error/loading details, focus entry,
  Tab wrapping, Escape restoration, folder/general controls, environment locks,
  help expansion and phone touch targets. No production media/config used.
- Documentation: [UI guide](../docs/ui.md), [configuration saving](../docs/configuration.md#saving-from-the-ui)
  and [settings coverage](../docs/settings-coverage.md). Release: `v2.0.16`.
- Implementation PR: #72; merge commit: `16c614e2dc32014b14492a1ecd8a8c61a36a25be`. Final PR checks passed:
  https://github.com/TheBadFella/UnpackUI/actions/runs/40941551852 and
  https://github.com/TheBadFella/UnpackUI/actions/runs/40941551829.
- Hosted Linux generation/tests, Linux/macOS/FreeBSD lint, frontend checks
  and the external archive integration suite passed on the final PR head.
- npm audit reports two low and two moderate dependency findings; no forced
  major-version changes were applied during the sync.

## Baseline recorded 2026-10-25

- Last reviewed upstream commit: `3ff4271eadae0c294e354458a86ca5f15be05e37`
  (PR #822, Update dependency rapidoc to v11).
- Fork parent: `ac33e09` / `cbb7adc`; previous upstream baseline: `1e60e6eb59af3fbbdd861321979f2506191d40b5`.
- Reviewed 52 upstream commits across 40 paths (1,973 insertions, 1,600 deletions). Merged in `540a91e`.
- Adopted upstream `preserve_exec` mode for watched folders (`pkg/folders/config.go`, `pkg/unpackerr/folder.go`,
  `pkg/unpackerr/configdump.go`, `pkg/unpackerr/openapi.json`, `pkg/configdef/definitions.yml`,
  `FoldersForm.svelte`, types, and locales).
- Adopted upstream xtractr v0.8.0, bytefmt v0.94.0, RapiDoc v11, and the Vitest frontend unit test suite
  (`src/**/*.spec.ts`) along with workflow integration.
- Adopted dependency updates: zod 4.6.7, fast-uri 3.1.9, undici, @scalar/openapi-parser, phosphor-svelte 3.1.1,
  vite 8.3.4, svelte 5.57.2, @types/node 24.19.2, golangci-lint v2.15, goreleaser-action v7.3.0,
  golang.org/x/net v0.61.0, and golang:1.27-alpine Docker digest.
- Preserved fork contracts:
  - Custom UI layouts, branding, dark/light themes, and dashboard/history views.
  - Detail modal dialog and regression tests (`tests/detail-modal-regression.mjs`).
  - Watched folder recovery, incomplete download deferral, and recursive folder semantics.
  - Fork release workflow and multi-architecture Docker builds (`ghcr.io/thebadfella/unpackui`).
  - Legacy `UN_FOLDERS_INTERVAL` fallback, `webserver.api` and `webserver.ui` compatibility switches,
    and flat stats fields for dashboard clients.
- Passed: `go test -count=1 ./...` (all Go packages passed), `npm run check` (0 errors, 0 warnings),
  `npm test` (Vitest: 13/13 test files, 43/43 tests passed),
  `node --experimental-strip-types --test tests/detail-modal-regression.mjs` (7/7 tests passed),
  `npm run build` (clean Vite production build), `golangci-lint run ./...` (0 issues), and `git diff --check`.
- Fork merge commit: `540a91e`.
- Release: `v2.0.14` (`v2.0.13` sync + `v2.0.14` dark monospace theming retention in task details and global modals).
- Release: `v2.0.15` (expand extracted directory paths into individual leaf files for history, queue, and webhooks; fix watched folder `delete_files` and `delete_orig` cleanup deletion to purge leaf files and empty subdirectories up to tracked folder).

### Previous baseline recorded 2026-10-13

- Last reviewed upstream commit: `1e60e6eb59af3fbbdd861321979f2506191d40b5`
  (PR #805, `code.cloudfoundry.org/bytefmt` to v0.93.0).
- Fork parent: `420e30aab82332e63bc62b7634cf1ebeea08cd33`; previous upstream baseline: `5203befbc0c3e06c05e7d8a3b56855f16998bcd0`.
- Reviewed 2 upstream commits across 2 paths (`go.mod`, `go.sum`). Merged in `bbd4ea2`.
- Converted below-table task details panel into shared modal dialog (`TaskDetails.svelte`)
  across both Queue and History tables. Preserved dark/light themes, typography, and layout.
- Adopted upstream full-item loading via `GET /api/queue/item`, one-shot completion refetch,
  and text file downloads via Blob API while keeping fork marker-file filtering (`visibleFiles`),
  progress/speed/ETA summaries, and recovery contracts.
- Passed: `go test -count=1 ./...`, `npm run check` (0 errors, 0 warnings), `npm run build`,
  `node --experimental-strip-types --test tests/detail-modal-regression.mjs` (7/7 tests),
  headless Chromium browser verification (focus restoration, Escape/Enter dismiss/open,
  scheduled cleanup display, marker file filtering, row-action isolation, and 375px mobile layout),
  and `git diff --check`.
- Fork merge commit: `bbd4ea2`.
- PR #63 and Dependabot PRs #58–#62 and #64 were merged in `fb5e7a1`.
  Hosted test/lint run `38395645849` and integration run `38395645840` passed.
  Startup now seeds restored folder history before recovery, and per-archive
  work checks wait extensions in its containing directory. The integration
  suite is pinned and its two directory-tracking expectations are adapted to
  the fork's recursive, per-archive contract without skipping tests.
- Release: `v2.0.12`. Docker build dependencies use verified registry mirrors
  after the v2.0.11 QEMU pull was rate-limited.

## Baseline recorded 2026-10-09

- Last reviewed upstream commit: `5203befbc0c3e06c05e7d8a3b56855f16998bcd0`
  (PR #801, Dockerfile syntax digest update).
- Fork parent: `7b7b0d4`; previous upstream baseline: `c88fb41`.
- Reviewed 29 upstream commits across 33 paths (738 insertions, 303 deletions).
- Adopted upstream Lidarr APE/CUE output format and compression settings,
  extraction helpers, validation, API/schema, and settings controls. Extraction
  uses upstream code without a parallel fork implementation.
- Adopted xtractr's nested-archive fixes, Starr expansion-ratio cap of 7.5,
  Go 1.27.1 and dependency updates, frontend lockfile updates, and Docker pins.
- Retained fork history fields, compatibility aliases, recursive watch tracking,
  interrupted extraction cleanup, recovery state, webhook behavior, dashboard
  layout, and fork branding. Upstream's new delta does not replace these contracts.
- Resolved conflicts in history serialization, README, INTERNALS, and CI.
  History writes and restores the new APE fields alongside fork fields.
  Added a regression test for that round trip.
- Kept `web-ui` in CI triggers, enabled the upstream integration workflow for
  the fork branch, and removed a stale matrix condition that skipped npm check.
- Passed: `go test ./...`, uncached folder/hook/unpackerr tests,
  `golangci-lint run ./...` (installed v2.13.0), `npm ci`,
  `npm run check` (zero errors/warnings), `npm run build`,
  `go generate ./...` using Git's installed shell, and diff checks.
  Generated timestamps were restored; the generated October release note was kept.
- Not run locally: hosted Linux/macOS lint v2.14 and the external Linux archive
  integration suite. Both remain CI gates.
- Dependency audit: source-map-js has one high advisory; esbuild/svelte-i18n
  have two moderate advisories. No forced dependency changes were made.
- Fork merge commit: `5804768c31e7694511559cedfe89678da86ab452`.
- Release: `v2.0.11`; tag publication runs the fork Docker workflow for
  `linux/amd64` and `linux/arm64`.

### Previous baseline recorded 2026-09-27

- Last reviewed upstream commit: `c88fb41850cd8b625639e4d3fd54120e13badfa6`
  (PR #787, Alpine 3.24 digest update).
- Fork parent: `a11d31077e894e5c67027c88bedb15fbc1fb563a` (v2.0.9)
  with upstream parent `c0dfb1b`.
- Merge commit: `1a9873b` with upstream parent `c88fb41`. The merge was clean.
- New upstream history since `c0dfb1b`: two commits (Renovate commit and its
  merge commit), changing only `Dockerfile` and
  `init/docker/Dockerfile.goreleaser`.
- Adopted Alpine digest `sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6`
  in both files. Kept the fork's source-build Dockerfile and GoReleaser
  executable-copy instructions. The fork delta is 96 paths after the merge.
- Local `go test ./...`, `golangci-lint run ./...`, frontend check/build, and
  registry inspection of the pinned Alpine manifest passed. The Linux PR and
  release workflows remain the generation and multi-platform image gates.

### Previous baseline recorded 2026-09-24

- Upstream: `https://github.com/Unpackerr/unpackerr.git`, `main`
- Fork: `https://github.com/TheBadFella/UnpackUI.git`, `web-ui`
- Last reviewed upstream commit: `c0dfb1b2af6760d2f7f969be264cd4d821b75d06`
  (`c0dfb1b`, PR #786, buildx bump)
- Fork parent: `2a910166cffea8bb484dca21815d7e59b6fe183f` (`2a91016`)
  with upstream parent `c0dfb1b`
- Merge base at review: `git merge-base HEAD upstream/main` = `c0dfb1b`.
  Upstream was fully merged; `HEAD..upstream/main` is empty.
- Upstream delta reviewed since previous upstream parent
  `c08b0bff7fe44cd39a342f7fc8b3df1af0b23c6e`: 30 commits across PRs #774,
  #775, #777, #778, #781, #784, #785, and #786 (38 files, 2,142 insertions and
  280 deletions).
- Current fork delta after the merge: `git diff upstream/main..HEAD` is the
  fork's intentional product, recovery, UI, documentation, test, and workflow
  surface (94 paths; 8,709 insertions and 1,558 deletions at this baseline).
  Review that delta separately from new upstream commits; it is not a reason to
  re-review old upstream history.

### Previous baseline recorded 2026-09-21
- Baseline SHA: `c08b0bff7fe44cd39a342f7fc8b3df1af0b23c6e` (`c08b0bf`)
- Fork merge commit: `cce412e3607b6bfbfa5f6928cea35f4eaf0942e4` (`cce412e`)

### Previous baseline recorded 2026-09-21 (earlier)
- Baseline SHA: `5f8acf1d9551941ef22bb33284d36c0d2352fb51` (`5f8acf1`)
- Fork merge commit: `6bac2f824b7adb1ab72bb1a5205bb5cfb560dab9` (`6bac2f8`)

### Previous baseline recorded 2026-09-18
- Baseline SHA: `e5c47abbcdb95e5147e4ecfb685be27b751ad4c6` (`e5c47ab`)
- Fork merge commit: `3eda941cc31368c8330c2e8ff160536ea8931f19` (`3eda941`)

### Fork-modified file inventory

```text
.agents/repository-context.md
.agents/skill-provenance.md
.agents/skills/change-unpackui/SKILL.md
.agents/skills/unslop-unpackui/SKILL.md
.agents/skills/verify-unpackui/SKILL.md
.agents/upstream-sync-ledger.md
.commandcode/taste/taste.md
.gitattributes
.github/copilot-instructions.md
.github/dependabot.yml
.github/workflows/cleanup-images.yml
.github/workflows/codetests.yml
.github/workflows/fork.yml
.github/workflows/inttest.yml
.github/workflows/release.yml
.gitignore
.golangci.yml
AGENTS.md
AUDIT_REPORT.md
INTERNALS.md
README.md
docs/configuration.md
docs/environment-variables.md
docs/notifications.md
docs/setup.md
docs/ui.md
examples/MANUAL.md
examples/docker-compose.yml
examples/unpackerr.conf.example
frontend/src/App.svelte
frontend/src/app.css
frontend/src/components/Hint.svelte
frontend/src/components/Nav.svelte
frontend/src/components/TaskDetails.svelte
frontend/src/lib/api.ts
frontend/src/lib/columns.ts
frontend/src/lib/dashboard.ts
frontend/src/lib/format.ts
frontend/src/lib/i18n/locales/el.json
frontend/src/lib/i18n/locales/en.json
frontend/src/lib/i18n/locales/es.json
frontend/src/lib/i18n/locales/nl.json
frontend/src/lib/types.ts
frontend/src/pages/Dashboard.svelte
frontend/src/pages/History.svelte
frontend/src/pages/Login.svelte
frontend/src/pages/Settings.svelte
frontend/src/pages/System.svelte
frontend/src/pages/settings/FoldersForm.svelte
go.sum
init/docker/Dockerfile.goreleaser
pkg/configdef/definitions.yml
pkg/folders/config.go
pkg/folders/folder_test.go
pkg/folders/watch.go
pkg/hooks/sample.go
pkg/hooks/templates.go
pkg/ui/UnpackUI.png
pkg/unpackerr/api.go
pkg/unpackerr/api_test.go
pkg/unpackerr/apps.go
pkg/unpackerr/cnfgfile.go
pkg/unpackerr/cnfgfile_fork_test.go
pkg/unpackerr/cnfgfile_test.go
pkg/unpackerr/configapi.go
pkg/unpackerr/configdump.go
pkg/unpackerr/configput.go
pkg/unpackerr/configput_test.go
pkg/unpackerr/duration.go
pkg/unpackerr/folder.go
pkg/unpackerr/folder_recursion_test.go
pkg/unpackerr/folder_test.go
pkg/unpackerr/folder_track_test.go
pkg/unpackerr/folder_wait_test.go
pkg/unpackerr/historyfile.go
pkg/unpackerr/historyfile_test.go
pkg/unpackerr/historyrestore.go
pkg/unpackerr/historyrestore_test.go
pkg/unpackerr/historyrestore_windows_test.go
pkg/unpackerr/logs.go
pkg/unpackerr/metrics.go
pkg/unpackerr/openapi.json
pkg/unpackerr/progress.go
pkg/unpackerr/queue_actions.go
pkg/unpackerr/queue_actions_test.go
pkg/unpackerr/recovery.go
pkg/unpackerr/recovery_test.go
pkg/unpackerr/recovery_windows_test.go
pkg/unpackerr/start.go
pkg/unpackerr/tray.go
pkg/unpackerr/webserver.go
pkg/unpackerr/webserver_test.go
settings.sh
tests/README.md
tests/add-dummy-entry.ps1
tests/detail-modal-regression.mjs
tests/run-codetests.ps1
tests/run-codetests.sh
tests/start-local.ps1
tests/stop-local.ps1
tests/upstream-inttest.patch
```

### Merge decisions and invariants

- Adopted upstream PR #774 / #775 (`HooksForm.svelte` in-app setup guides, automatic template detection, and custom headers `[webhook.headers]`), `pkg/hooks/headers.go`, and header redaction on config read/test.
- Adopted upstream PR #778 (`folderExcludeSuffixes(cfg)`), keeping watched archives searchable when `disable_recursion` is active while preserving fork's incomplete folder recovery and recursive watch logic.
- Adopted upstream PR #781 (`X-Api-Key` header migration for Notifiarr webhooks and URL key sanitization) via upstream's `pkg/hooks/notifiarr.go`.
- Adopted upstream PR #771 (`HookFail` tracking across restart and retries), PR #772 (webhook template profiles `ntfy`, `Apprise`, `Mattermost`), and PR #773 (Discord/Telegram message-id editing), integrating cleanly with fork's queue item/history fields (`Title`, `Reason`, `ArchiveFiles`, `DeleteAt`) and recovery channels.
- `pkg/hooks/http.go` was brought back into full alignment with upstream as delivery logic moved upstream to `pkg/hooks/update.go`.
- Adopted upstream's per-watch-path folder pollers and non-recursive poller
  roots, while retaining the fork's recursive discovery, nested-watch handling,
  path confinement, and incomplete-extraction recovery.
- Kept the legacy global `UN_FOLDERS_INTERVAL` as a compatibility fallback for
  old config files and environment overlays. An explicit per-folder interval
  takes precedence; an unset per-folder interval still allows fsnotify.
- Kept root-safe path matching and selected the most-specific watch root for
  nested recovery and startup scans. The watch root itself is not treated as an
  extraction output.
- Preserved archive/path confinement, recovery-state handling, queue/history
  behavior, and webhook deduplication. Generated config/API examples and the
  frontend schema remain synchronized with the definitions.

### Compatibility surface remaining after the web UI migration

- `webserver.api` and `webserver.ui` are parsed for existing TOML and
  environment configurations, but do not gate the dashboard or authenticated
  API. `listen_addr` controls whether the web server runs.
- Global `UN_FOLDERS_INTERVAL` remains a fallback for older configurations;
  an explicit per-folder interval takes precedence.
- Array-shaped Starr, folder, and hook configurations, the singular Starr
  `path` alias, queue progress aliases, and old hash-route redirects remain for
  existing files, API clients, and bookmarks.
- Homepage-compatible stats fields, persisted recovery/history migration,
  embedded-SPA build fallbacks, platform-specific path handling, and the
  fork/upstream release workflow split are active contracts, not legacy code.

### Evidence and limitations

- Targeted folder, recovery, tracking, configuration, and poller tests passed;
  the full Go suite, frontend check/build, Linux lint, generated-file checks,
  and a temporary poller event probe passed for this merge review.
- Windows `go generate ./...` was not independently run because the frontend
  generator requires `sh`, which is not on the Windows PATH. Generated
  timestamp-only changes were restored after inspection.

### Release baseline

- The next safe patch release is `v2.0.14`; existing `v2.0.13` remains untouched.
- The Git tag is the product version source: `settings.sh` and the Makefile
  derive build metadata from tags. The private frontend package version and
  embedded terminal-notifier metadata are not product release references.

## Repeatable sync procedure

Run from a clean `web-ui` worktree. Replace the recorded SHA in `lastReviewed`
with the value in this file before starting the next review.

```powershell
git fetch --prune upstream main
git fetch --prune origin web-ui --tags
git status --short --branch
$lastReviewed = '3ff4271eadae0c294e354458a86ca5f15be05e37'
git rev-parse upstream/main
git rev-list --left-right --count "$lastReviewed..upstream/main"
git log --oneline --decorate "$lastReviewed..upstream/main"
git diff --stat "$lastReviewed..upstream/main"
git diff --name-status "$lastReviewed..upstream/main"
git merge-base HEAD upstream/main
git diff --stat upstream/main..HEAD
git log --first-parent --oneline upstream/main..HEAD
```

Review only the commits and paths in the `lastReviewed..upstream/main` output,
then compare their callers and tests against the invariant list above. Merge
with `git merge --no-ff --no-edit upstream/main`, resolve conflicts in favor of
the fork contracts where needed, run the relevant targeted tests followed by
the repository release checks, and inspect `git diff --check` plus generated
files. Update this ledger with the new upstream SHA, merge commit, delta,
decisions, evidence, and limitations only after verification passes.

For a Docker release, choose a new patch tag; never move or recreate an
existing tag. Confirm the new tag is absent on `origin`, commit the ledger and
release changes, then push the branch and tag:

```powershell
git ls-remote --exit-code --tags origin 'refs/tags/v<new-version>'
# The command above must fail because the tag does not exist.
git tag -a v<new-version> -m "UnpackUI v<new-version>"
git push origin web-ui
git push origin v<new-version>
gh run list --workflow fork.yml --limit 1 --json databaseId,url,status,conclusion,headSha
gh run watch <run-id> --exit-status
```

On this fork, `fork.yml` runs for `v*` tags and publishes
`ghcr.io/thebadfella/unpackui` for `linux/amd64` and `linux/arm64`, including
the semver tag, `latest`, and a commit tag. Record the workflow URL, terminal
status, image tags, and any unavailable external check in the release report.
