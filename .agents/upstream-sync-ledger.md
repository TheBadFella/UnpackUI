# Upstream sync ledger

This ledger records the last reviewed `upstream/main` commit so future syncs can
inspect only the upstream commits that arrived after that point. It is a review
baseline, not a replacement for the repository contracts in
[`repository-context.md`](repository-context.md).

## Baseline recorded 2026-09-18

- Upstream: `https://github.com/Unpackerr/unpackerr.git`, `main`
- Fork: `https://github.com/TheBadFella/UnpackUI.git`, `web-ui`
- Last reviewed upstream commit: `e5c47abbcdb95e5147e4ecfb685be27b751ad4c6`
  (`e5c47ab`, PR #763, per-folder polling)
- Fork merge commit: `3eda941cc31368c8330c2e8ff160536ea8931f19`
  (`3eda941`), with fork parent `95f728e` and upstream parent `e5c47ab`
- Merge base at review: `git merge-base HEAD upstream/main` = `e5c47ab`.
  Upstream was fully merged; `HEAD..upstream/main` was empty.
- Upstream delta reviewed since the previous upstream parent
  `076719bb5b336add6beee7d12a4091ebcf2f801e`: four commits, 24 files,
  531 insertions and 193 deletions. The fork-side merge result changed 25
  files, with 750 insertions and 211 deletions relative to `95f728e`.
- Current fork delta after the merge: `git diff upstream/main..HEAD` is the
  fork's intentional product, recovery, UI, documentation, test, and workflow
  surface (92 paths; 8,084 insertions and 1,599 deletions at this baseline).
  Review that delta separately from new upstream commits; it is not a reason to
  re-review old upstream history.

### Merge decisions and invariants

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

### Evidence and limitations

- Targeted folder, recovery, tracking, configuration, and poller tests passed;
  the full Go suite, frontend check/build, Linux lint, generated-file checks,
  and a temporary poller event probe passed for this merge review.
- Windows `go generate ./...` was not independently run because the frontend
  generator requires `sh`, which is not on the Windows PATH. Generated
  timestamp-only changes were restored after inspection.

### Release baseline

- The next safe patch release is `v2.0.3`; existing `v2.0.2` remains untouched.
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
