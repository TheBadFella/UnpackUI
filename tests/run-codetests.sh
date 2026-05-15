#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

run_step() {
  local name="$1"
  shift
  printf '==> %s\n' "$name"
  "$@"
}

run_step "go generate ./..." go generate ./...
run_step "go test ./pkg/..." go test ./pkg/...
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    printf '==> GOOS=linux go generate ./... skipped\n'
    printf '    GOOS=linux go generate ./... cannot execute generated Linux binaries on native Windows. Run this script in Linux/WSL for the exact CI flow.\n'
    ;;
  *)
    run_step "GOOS=linux go generate ./..." env GOOS=linux go generate ./...
    ;;
esac

if [[ "${1:-}" == "--skip-lint" ]]; then
  printf '==> golangci-lint skipped\n'
  exit 0
fi

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if [[ "${1:-}" != "--run-lint-on-windows" ]]; then
      printf '==> golangci-lint skipped on native Windows\n'
      printf '    golangci-lint formatters are CI/Linux-sensitive in this repo and produce noisy native Windows gci/gofmt results. Run from Linux/WSL for the exact CI lint step.\n'
      printf '    Pass --run-lint-on-windows to force the native Windows lint run.\n'
      exit 0
    fi
    ;;
esac

lint_version="v2.11.4"
printf '==> golangci-lint run (%s)\n' "$lint_version"

if command -v golangci-lint >/dev/null 2>&1; then
  lint_bin="golangci-lint"
else
  tools_dir="$(pwd)/tests/.bin"
  mkdir -p "$tools_dir"
  lint_bin="${tools_dir}/golangci-lint"
  if [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* || "$(uname -s)" == CYGWIN* ]]; then
    lint_bin="${lint_bin}.exe"
  fi

  if [[ ! -x "$lint_bin" ]]; then
    printf '==> install golangci-lint (%s)\n' "$lint_version"
    GOBIN="$tools_dir" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${lint_version}"
  fi
fi

GOOS=linux "$lint_bin" run
