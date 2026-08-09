#!/usr/bin/env bash
# Download official yt-dlp release binaries for the current (or target) platform.
#
# Usage:
#   ./scripts/fetch-ytdlp.sh              # → ~/.cache/vYtDL/yt-dlp
#   ./scripts/fetch-ytdlp.sh --embed      # → internal/ytdlpbin/binaries/yt-dlp.bin
#   GOOS=linux GOARCH=amd64 ./scripts/fetch-ytdlp.sh --embed
#   ./scripts/fetch-ytdlp.sh --outdir ./dist/tools

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

MODE="cache"
OUTDIR=""
TAG="${YTDLP_TAG:-latest}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --embed) MODE="embed"; shift ;;
    --outdir) OUTDIR="$2"; shift 2 ;;
    --tag) TAG="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

GOOS_VAL="${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
case "${GOOS_VAL}" in
  darwin|linux|windows) ;;
  mingw*|msys*|cygwin*) GOOS_VAL="windows" ;;
  *) echo "unsupported GOOS=${GOOS_VAL}" >&2; exit 1 ;;
esac

GOARCH_VAL="${GOARCH:-$(uname -m)}"
case "${GOARCH_VAL}" in
  x86_64|amd64) GOARCH_VAL="amd64" ;;
  aarch64|arm64) GOARCH_VAL="arm64" ;;
  armv7l) GOARCH_VAL="arm" ;;
esac

asset=""
case "${GOOS_VAL}/${GOARCH_VAL}" in
  darwin/*) asset="yt-dlp_macos" ;;
  linux/amd64) asset="yt-dlp_linux" ;;
  linux/arm64) asset="yt-dlp_linux_aarch64" ;;
  windows/amd64) asset="yt-dlp.exe" ;;
  windows/arm64) asset="yt-dlp_arm64.exe" ;;
  *) echo "unsupported target ${GOOS_VAL}/${GOARCH_VAL}" >&2; exit 1 ;;
esac

if [[ "${TAG}" == "latest" ]]; then
  URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/${asset}"
else
  URL="https://github.com/yt-dlp/yt-dlp/releases/download/${TAG}/${asset}"
fi

if [[ "${MODE}" == "embed" ]]; then
  DEST_DIR="${PROJECT_DIR}/internal/ytdlpbin/binaries"
  mkdir -p "${DEST_DIR}"
  DEST="${DEST_DIR}/yt-dlp.bin"
elif [[ -n "${OUTDIR}" ]]; then
  mkdir -p "${OUTDIR}"
  if [[ "${GOOS_VAL}" == "windows" ]]; then
    DEST="${OUTDIR}/yt-dlp.exe"
  else
    DEST="${OUTDIR}/yt-dlp"
  fi
else
  CACHE="${XDG_CACHE_HOME:-${HOME}/.cache}/vYtDL"
  mkdir -p "${CACHE}"
  if [[ "${GOOS_VAL}" == "windows" ]]; then
    DEST="${CACHE}/yt-dlp.exe"
  else
    DEST="${CACHE}/yt-dlp"
  fi
fi

echo "Downloading ${URL}"
echo "         → ${DEST}"
TMP="${DEST}.tmp"
download() {
  local url="$1"
  if [[ -n "${VYTDL_YTDLP_MIRROR:-}" ]]; then
    if [[ "${VYTDL_YTDLP_MIRROR}" == */ ]]; then
      url="${VYTDL_YTDLP_MIRROR}${asset}"
    else
      url="${VYTDL_YTDLP_MIRROR}"
    fi
    echo "Using mirror: ${url}"
  fi
  curl -fL --connect-timeout 30 --retry 3 --retry-delay 2 --max-time 600 -o "${TMP}" "${url}"
}

if ! download "${URL}"; then
  echo "Primary download failed; trying ghproxy.net mirror..." >&2
  VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/" download "${URL}"
fi
chmod +x "${TMP}" || true
mv "${TMP}" "${DEST}"
ls -lh "${DEST}"
echo "OK"
