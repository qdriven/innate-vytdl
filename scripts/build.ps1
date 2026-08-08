param(
    [string]$OutputDir = "dist",
    [string]$AppName = "vYtDL"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectDir = (Resolve-Path (Join-Path $scriptDir "..")).Path
$targetDir = Join-Path $projectDir $OutputDir

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

$targets = @(
    @{ GOOS = "darwin"; GOARCH = "amd64" },
    @{ GOOS = "darwin"; GOARCH = "arm64" },
    @{ GOOS = "linux"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64" }
)

$oldCgo = $env:CGO_ENABLED
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH

Push-Location $projectDir
try {
    foreach ($target in $targets) {
        $goos = $target.GOOS
        $goarch = $target.GOARCH
        $ext = if ($goos -eq "windows") { ".exe" } else { "" }
        $outFile = Join-Path $targetDir "$AppName-$goos-$goarch$ext"

        Write-Host "Building $outFile"
        $env:CGO_ENABLED = "0"
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        go build -trimpath -ldflags "-s -w" -o $outFile .
    }
}
finally {
    $env:CGO_ENABLED = $oldCgo
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    Pop-Location
}

Write-Host "Build artifacts are in $targetDir"
