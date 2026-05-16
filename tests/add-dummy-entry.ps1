param(
    [string]$Config = ".\unpackerr.local.conf",
    [string]$WatchPath = "",
    [string]$Name = "",
    [switch]$NoTimestamp
)

$ErrorActionPreference = 'Stop'

function Get-ConfiguredWatchPath {
    param([string]$ConfigPath)

    if (!(Test-Path $ConfigPath)) {
        return ".\tmp\watch"
    }

    $folderSeen = $false
    foreach ($line in Get-Content $ConfigPath) {
        if ($line -match '^\s*\[\[folder\]\]\s*$') {
            $folderSeen = $true
            continue
        }

        if ($folderSeen -and $line -match '^\s*path\s*=\s*"([^"]+)"') {
            return $Matches[1]
        }
    }

    return ".\tmp\watch"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$configPath = Join-Path $repoRoot $Config
if ([string]::IsNullOrWhiteSpace($WatchPath)) {
    $WatchPath = Get-ConfiguredWatchPath -ConfigPath $configPath
}

if ([string]::IsNullOrWhiteSpace($Name)) {
    $Name = "Berlin and the Lady with an Ermine (2026) S01 2160p NF 10bit WEB-DL SDR HEVC H.265 (HHWEB-Tonicboy)"
}

if (!$NoTimestamp) {
    $Name = "$Name-$(Get-Date -Format 'HHmmss')"
}

$resolvedWatch = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($WatchPath)
New-Item -ItemType Directory -Force -Path $resolvedWatch | Out-Null

$stagingPath = Join-Path $resolvedWatch $Name
$archivePath = Join-Path $resolvedWatch "$Name.zip"
$samplePath = Join-Path $stagingPath "sample.txt"

Remove-Item -LiteralPath $stagingPath, $archivePath -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $stagingPath | Out-Null
Set-Content -LiteralPath $samplePath -Value "Local UnpackUI test archive $(Get-Date -Format o)"
Compress-Archive -LiteralPath $samplePath -DestinationPath $archivePath -Force
Remove-Item -LiteralPath $stagingPath -Recurse -Force

$archive = Get-Item -LiteralPath $archivePath
Write-Host "==> Created dummy archive" -ForegroundColor Green
Write-Host "    $($archive.FullName)"
Write-Host "    Open: http://127.0.0.1:5656/"
