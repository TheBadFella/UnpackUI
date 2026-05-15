param(
    [switch]$SkipLint,
    [switch]$Strict,
    [switch]$RunLintOnWindows
)

$ErrorActionPreference = 'Stop'

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Command
    )

    Write-Host "==> $Name" -ForegroundColor Cyan
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

Invoke-Step "go generate ./..." {
    go generate ./...
}

Invoke-Step "go test ./pkg/..." {
    go test ./pkg/...
}

if ($IsWindows -or $env:OS -eq 'Windows_NT') {
    $message = "GOOS=linux go generate ./... cannot execute generated Linux binaries on native Windows. Run this script in Linux/WSL for the exact CI flow."
    if ($Strict) {
        throw $message
    }
    Write-Host "==> GOOS=linux go generate ./... skipped" -ForegroundColor Yellow
    Write-Host "    $message" -ForegroundColor Yellow
} else {
    Invoke-Step "GOOS=linux go generate ./..." {
        $previousGOOS = $env:GOOS
        try {
            $env:GOOS = 'linux'
            go generate ./...
        } finally {
            $env:GOOS = $previousGOOS
        }
    }
}

if ($SkipLint) {
    Write-Host "==> golangci-lint skipped" -ForegroundColor Yellow
    exit 0
}

if (($IsWindows -or $env:OS -eq 'Windows_NT') -and !$RunLintOnWindows) {
    $message = "golangci-lint formatters are CI/Linux-sensitive in this repo and produce noisy native Windows gci/gofmt results. Run from Linux/WSL for the exact CI lint step."
    if ($Strict) {
        throw $message
    }
    Write-Host "==> golangci-lint skipped on native Windows" -ForegroundColor Yellow
    Write-Host "    $message" -ForegroundColor Yellow
    Write-Host "    Pass -RunLintOnWindows to force the native Windows lint run." -ForegroundColor Yellow
    exit 0
}

$lintVersion = 'v2.11.4'
$lintCommand = Get-Command golangci-lint -ErrorAction SilentlyContinue
$lintExe = $null

if ($lintCommand) {
    $lintExe = $lintCommand.Source
} else {
    $toolsDir = Join-Path $PSScriptRoot '.bin'
    New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null

    $exeName = 'golangci-lint'
    if ($IsWindows -or $env:OS -eq 'Windows_NT') {
        $exeName += '.exe'
    }

    $lintExe = Join-Path $toolsDir $exeName
    if (!(Test-Path $lintExe)) {
        Invoke-Step "install golangci-lint ($lintVersion)" {
            $previousGOBIN = $env:GOBIN
            $previousGOOS = $env:GOOS
            try {
                $env:GOBIN = $toolsDir
                Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
                go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$lintVersion"
            } finally {
                $env:GOBIN = $previousGOBIN
                $env:GOOS = $previousGOOS
            }
        }
    }
}

Invoke-Step "golangci-lint run ($lintVersion)" {
    $previousGOOS = $env:GOOS
    try {
        $env:GOOS = 'linux'
        & $lintExe run
    } finally {
        $env:GOOS = $previousGOOS
    }
}
