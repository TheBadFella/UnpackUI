param(
    [string]$Config = ".\unpackerr.local.conf",
    [int]$Port = -1,
    [string]$PidFile = "",
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'

function Write-Status {
    param(
        [string]$Message,
        [string]$Color = "Cyan"
    )

    if (!$Quiet) {
        Write-Host $Message -ForegroundColor $Color
    }
}

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

function Stop-LocalProcess {
    param([int]$ProcessId)

    if ($ProcessId -le 0 -or $ProcessId -eq $PID) {
        return
    }

    $process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (!$process) {
        return
    }

    Write-Status "==> Stopping PID $ProcessId"
    Stop-Process -Id $ProcessId -Force -ErrorAction Stop
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$configPath = Join-Path $repoRoot $Config
if ($Port -lt 0 -and (Test-Path $configPath)) {
    $Port = Get-ConfiguredPort -ConfigPath $configPath
}

if ([string]::IsNullOrWhiteSpace($PidFile)) {
    $PidFile = Join-Path $repoRoot "tmp\unpackerr.local.pid"
}

if (Test-Path $PidFile) {
    $savedPid = (Get-Content $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1)
    if ($savedPid -match '^\d+$') {
        Stop-LocalProcess -ProcessId ([int]$savedPid)
    }

    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
}

if ($Port -gt 0) {
    $listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique

    foreach ($listenerPid in $listeners) {
        Stop-LocalProcess -ProcessId ([int]$listenerPid)
    }
}

if (!$Quiet) {
    if ($Port -gt 0) {
        Write-Host "==> Port $Port is free" -ForegroundColor Green
    } else {
        Write-Host "==> Local UnpackUI process stopped" -ForegroundColor Green
    }
}
