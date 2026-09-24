# Upstream sync ledger

This ledger records the last reviewed `upstream/main` commit so future syncs can
inspect only the upstream commits that arrived after that point. It is a review
baseline, not a replacement for the repository contracts in
[`repository-context.md`](repository-context.md).

## Baseline recorded 2026-09-24

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
tests/run-codetests.ps1
tests/run-codetests.sh
tests/start-local.ps1
tests/stop-local.ps1
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

- The next safe patch release is `v2.0.7`; existing `v2.0.6` remains untouched.
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
$lastReviewed = 'e5c47abbcdb95e5147e4ecfb685be27b751ad4c6'
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
