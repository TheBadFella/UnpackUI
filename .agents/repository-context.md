# UnpackUI engineering context

Maintain extraction, watched folders, recovery and web controls.

Stack observed on 2026-09-06: Go Unpackerr fork + web UI + platform packaging. Recheck manifests and scoped instructions when the implementation changes.

## Read for the affected area

- `go.mod`
- `pkg/unpackerr`
- `pkg/unpackerr/webui_test.go`
- `pkg/unpackerr/recovery_test.go`
- `tests/run-codetests.ps1`
- `tests/run-codetests.sh`
- `.github/workflows/codetests.yml`

## Contracts to preserve

- Preserve archive/path confinement, recursive folder semantics, incomplete extraction recovery and webhook deduplication.
- Use temporary archives and output folders; never point tests or local executable at production media/config.
- Keep the upstream Go module identity unless migration is requested. Linux generation/lint and Windows checks are distinct.

## Verification recipes

These commands were found in project instructions, manifests, tests or CI and reviewed for task fit. Their inclusion does not mean they ran or passed during the skill audit. Inspect test fixtures and environment prerequisites before execution. Run only checks relevant to the change; keep any stricter repository release gate.

| Working directory | Command | Purpose / condition |
|---|---|---|
| root | `go test ./...` | Go regression suite |
| root on Windows | `pwsh -NoProfile -File tests/run-codetests.ps1` | Generates code and may install tools; Windows reports Linux-only skips |
| root on Linux/WSL | `bash tests/run-codetests.sh` | Inspect script before exact CI parity run |

## Observable proof

Assert final extracted files, path boundaries, recovery state and UI responses with fixture archives. go generate can mutate generated artifacts; inspect its diff. Do not report Windows skipped lint/generation as full CI success.

## Communication

Distinguish UnpackUI product and unpackerr module/service names. Preserve archive flags, configuration keys and queue state labels.
