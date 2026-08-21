# Cross-compile vYtDL for macOS / Linux / Windows.
#
# Usage:
#   .\scripts\build.ps1
#   .\scripts\build.ps1 -OutputDir dist -AppName vYtDL
#   .\scripts\build.ps1 -Only windows
#   .\scripts\build.ps1 -Only windows/amd64
#   $env:EMBED=1; .\scripts\build.ps1 -Only windows

param(
    [string]$OutputDir = "dist",
    [string]$AppName = "vYtDL",
    [string]$Only = "",
    [switch]$Embed
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Ignore parent go.work when this tree is checked out inside a monorepo.
if (-not $env:GOWORK) { $env:GOWORK = "off" }

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectDir = (Resolve-Path (Join-Path $scriptDir "..")).Path
$targetDir = Join-Path $projectDir $OutputDir

if ($Embed) { $env:EMBED = "1" }
$doEmbed = ($env:EMBED -eq "1")

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

$allTargets = @(
    @{ GOOS = "darwin"; GOARCH = "amd64" },
    @{ GOOS = "darwin"; GOARCH = "arm64" },
    @{ GOOS = "linux"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64" }
)

function Should-Build([string]$Goos, [string]$Goarch, [string]$OnlyFilter) {
    if ([string]::IsNullOrWhiteSpace($OnlyFilter)) { return $true }
    $tokens = $OnlyFilter.ToLowerInvariant() -split "," | ForEach-Object { $_.Trim() }
    foreach ($token in $tokens) {
        if ($token -eq $Goos -or
            $token -eq "$Goos/$Goarch" -or
            $token -eq "$Goos-$Goarch" -or
            $token -eq "${Goos}_$Goarch") {
            return $true
        }
    }
    return $false
}

$oldCgo = $env:CGO_ENABLED
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$built = 0

Push-Location $projectDir
try {
    foreach ($target in $allTargets) {
        $goos = $target.GOOS
        $goarch = $target.GOARCH
        if (-not (Should-Build $goos $goarch $Only)) { continue }

        $ext = if ($goos -eq "windows") { ".exe" } else { "" }
        $outFile = Join-Path $targetDir "$AppName-$goos-$goarch$ext"

        Write-Host "Building $outFile"

        $tags = @()
        if ($doEmbed) {
            Write-Host "  fetching yt-dlp for embed ($goos/$goarch)..."
            $env:GOOS = $goos
            $env:GOARCH = $goarch
            & bash (Join-Path $scriptDir "fetch-ytdlp.sh") --embed
            if ($LASTEXITCODE -ne 0) { throw "fetch-ytdlp.sh failed" }
            $tags = @("-tags", "embed_ytdlp")
        }

        $env:CGO_ENABLED = "0"
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        & go build -trimpath -ldflags "-s -w" @tags -o $outFile .
        if ($LASTEXITCODE -ne 0) { throw "go build failed for $goos/$goarch" }
        $built++
    }
}
finally {
    $env:CGO_ENABLED = $oldCgo
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    Pop-Location
}

if ($built -eq 0) {
    Write-Error "No targets matched -Only $Only. Examples: -Only windows  -Only windows/amd64"
    exit 1
}

Write-Host "Build artifacts are in $targetDir ($built target(s))"
if ($doEmbed) {
    Write-Host "Note: binaries were built with -tags embed_ytdlp (yt-dlp embedded)."
}
