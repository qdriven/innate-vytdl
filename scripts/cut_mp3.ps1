# cut_mp3.ps1 - Cut an MP3 file within a time range using FFmpeg
#
# Usage:
#   .\cut_mp3.ps1 -InputFile input.mp3 -StartTime 00:01:30 -EndTime 00:02:45 -OutputFile output.mp3
#   .\cut_mp3.ps1 -InputFile input.mp3 -StartTime 90 -Duration 60 -OutputFile output.mp3
#   .\cut_mp3.ps1 -InputFile input.mp3 -StartTime 00:01:30 -Duration 120

param(
    [Parameter(Mandatory=$true)]
    [Alias("i")]
    [string]$InputFile,

    [Parameter(Mandatory=$true)]
    [Alias("s")]
    [string]$StartTime,

    [Alias("e")]
    [string]$EndTime,

    [Alias("d")]
    [string]$Duration,

    [Alias("o")]
    [string]$OutputFile
)

# Validate input file
if (-not (Test-Path $InputFile)) {
    Write-Error "Input file not found: $InputFile"
    exit 1
}

# Check ffmpeg
if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
    Write-Error "ffmpeg not found. Please install ffmpeg first."
    Write-Host "  winget install Gyan.FFmpeg"
    exit 1
}

# Default output name
if (-not $OutputFile) {
    $base = [System.IO.Path]::GetFileNameWithoutExtension($InputFile)
    $dir = [System.IO.Path]::GetDirectoryName($InputFile)
    $OutputFile = Join-Path $dir "$base`_cut.mp3"
}

# Build ffmpeg arguments
$ffmpegArgs = @("-y", "-ss", $StartTime)

if ($EndTime) {
    $ffmpegArgs += @("-to", $EndTime)
} elseif ($Duration) {
    $ffmpegArgs += @("-t", $Duration)
}

$ffmpegArgs += @("-i", $InputFile, "-c", "copy", $OutputFile)

Write-Host "Cutting MP3..."
Write-Host "  Input:    $InputFile"
Write-Host "  Start:    $StartTime"
if ($EndTime) { Write-Host "  End:      $EndTime" }
if ($Duration) { Write-Host "  Duration: $Duration" }
Write-Host "  Output:   $OutputFile"
Write-Host ""

& ffmpeg @ffmpegArgs

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "Done! Output saved to: $OutputFile" -ForegroundColor Green
} else {
    Write-Error "ffmpeg failed with exit code $LASTEXITCODE"
    exit 1
}
