param(
    [string]$Config = ".\unpackerr.local.conf",
    [int]$Port = 0,
    [switch]$NoBuild,
    [switch]$FreePort
)

$ErrorActionPreference = 'Stop'

function Get-ConfiguredPort {
    param([string]$ConfigPath)

    if (!(Test-Path $ConfigPath)) {
        return 5656
    }

    $listenLine = Get-Content $ConfigPath |
        Where-Object { $_ -match '^\s*listen_addr\s*=' } |
        Select-Object -First 1

    if ($listenLine -and $listenLine -match ':(\d+)') {
        return [int]$Matches[1]
    }

    return 5656
}

function Test-PortListening {
    param([int]$LocalPort)

    $listener = Get-NetTCPConnection -LocalPort $LocalPort -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1

    return $null -ne $listener
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$configPath = Join-Path $repoRoot $Config
if (!(Test-Path $configPath)) {
    throw "Config file not found: $configPath"
}

if ($Port -le 0) {
    $Port = Get-ConfiguredPort -ConfigPath $configPath
}

$tmpDir = Join-Path $repoRoot "tmp"
$watchDir = Join-Path $tmpDir "watch"
$exePath = Join-Path $tmpDir "unpackerr.local.exe"
$pidFile = Join-Path $tmpDir "unpackerr.local.pid"
$stdoutLog = Join-Path $tmpDir "unpackerr.stdout.log"
$stderrLog = Join-Path $tmpDir "unpackerr.stderr.log"

New-Item -ItemType Directory -Force -Path $tmpDir, $watchDir | Out-Null

$stopScript = Join-Path $PSScriptRoot "stop-local.ps1"
if (Test-Path $stopScript) {
    if ($FreePort) {
        & $stopScript -Config $Config -Port $Port -PidFile $pidFile -Quiet
    } else {
        & $stopScript -PidFile $pidFile -Port 0 -Quiet
    }
}

if (Test-PortListening -LocalPort $Port) {
    throw "Port $Port is already in use. Run .\tests\stop-local.ps1 -Port $Port, or start with -FreePort."
}

if (!$NoBuild) {
    Write-Host "==> Building local unpackerr binary" -ForegroundColor Cyan
    go build -o $exePath .
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
} elseif (!(Test-Path $exePath)) {
    throw "No local binary found at $exePath. Run without -NoBuild first."
}

Remove-Item -LiteralPath $stdoutLog, $stderrLog -Force -ErrorAction SilentlyContinue

Write-Host "==> Starting UnpackUI on http://127.0.0.1:$Port/" -ForegroundColor Cyan
$previousUseGui = $env:USEGUI
try {
    $env:USEGUI = "false"
    $process = Start-Process `
        -FilePath $exePath `
        -ArgumentList @("-c", $configPath) `
        -WorkingDirectory $repoRoot `
        -RedirectStandardOutput $stdoutLog `
        -RedirectStandardError $stderrLog `
        -WindowStyle Hidden `
        -PassThru
} finally {
    $env:USEGUI = $previousUseGui
}

Set-Content -Path $pidFile -Value $process.Id

$deadline = (Get-Date).AddSeconds(15)
while ((Get-Date) -lt $deadline) {
    if ($process.HasExited) {
        throw "UnpackUI exited early with code $($process.ExitCode). See $stderrLog and $stdoutLog."
    }

    if (Test-PortListening -LocalPort $Port) {
        Write-Host "==> Started PID $($process.Id)" -ForegroundColor Green
        Write-Host "    UI: http://127.0.0.1:$Port/"
        Write-Host "    Logs: $stdoutLog, $stderrLog"
        Write-Host "    Stop: .\tests\stop-local.ps1"
        exit 0
    }

    Start-Sleep -Milliseconds 250
    $process.Refresh()
}

throw "Timed out waiting for port $Port to listen. See $stderrLog and $stdoutLog."
