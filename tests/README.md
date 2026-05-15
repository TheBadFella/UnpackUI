# Local Code Tests

These scripts mirror `.github/workflows/codetests.yml` for local runs.

PowerShell:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tests\run-codetests.ps1
```

Bash:

```bash
./tests/run-codetests.sh
```

The lint step uses a local `golangci-lint` binary when available. If it is not installed, the scripts install the pinned CI-compatible version into `tests/.bin` with:

```text
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
```

To run generate/tests without lint:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tests\run-codetests.ps1 -SkipLint
```

```bash
./tests/run-codetests.sh --skip-lint
```

On native Windows, the `GOOS=linux go generate ./...` step is skipped because Go builds Linux generator binaries that Windows cannot execute. Run the bash script from Linux or WSL for the exact CI flow. Use `-Strict` with the PowerShell script if you want Windows runs to fail instead of skipping that step.

Native Windows also skips `golangci-lint` by default because this repo's formatter checks are CI/Linux-sensitive and produce noisy `gci/gofmt` results on a Windows checkout. Use Linux/WSL for the exact lint run, or force the native Windows lint run with:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tests\run-codetests.ps1 -RunLintOnWindows
```
