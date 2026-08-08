#!/usr/bin/env bash

set -euo pipefail

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
  echo "Usage: $0 <video_url> <output_dir> [quality]" >&2
  exit 1
fi

VIDEO_URL="$1"
OUTPUT_DIR="$2"
QUALITY="${3:-1080}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if command -v yt-dlp >/dev/null 2>&1; then
  :
elif command -v youtube-dl >/dev/null 2>&1; then
  :
else
  echo "yt-dlp is not installed or not in PATH." >&2
  echo "Install it first, for example:" >&2
  echo "  macOS:  brew install yt-dlp" >&2
  echo "  Linux:  pipx install yt-dlp  (or pip install -U yt-dlp)" >&2
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

cd "${PROJECT_DIR}"
go run . download \
  --no-tui \
  --quality "${QUALITY}" \
  --output "${OUTPUT_DIR}" \
  "${VIDEO_URL}"
