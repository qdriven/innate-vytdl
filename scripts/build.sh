#!/usr/bin/env bash

set -euo pipefail

OUTPUT_DIR="${1:-dist}"
APP_NAME="${2:-vYtDL}"
EMBED="${EMBED:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

mkdir -p "${PROJECT_DIR}/${OUTPUT_DIR}"

TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

for target in "${TARGETS[@]}"; do
  read -r GOOS GOARCH <<<"${target}"
  EXT=""
  if [[ "${GOOS}" == "windows" ]]; then
    EXT=".exe"
  fi
  OUT_FILE="${PROJECT_DIR}/${OUTPUT_DIR}/${APP_NAME}-${GOOS}-${GOARCH}${EXT}"
  echo "Building ${OUT_FILE}"

  TAGS=""
  if [[ "${EMBED}" == "1" ]]; then
    echo "  fetching yt-dlp for embed (${GOOS}/${GOARCH})..."
    GOOS="${GOOS}" GOARCH="${GOARCH}" "${SCRIPT_DIR}/fetch-ytdlp.sh" --embed
    TAGS="-tags embed_ytdlp"
  fi

  # shellcheck disable=SC2086
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags "-s -w" ${TAGS} -o "${OUT_FILE}" "${PROJECT_DIR}"
done

echo "Build artifacts are in ${PROJECT_DIR}/${OUTPUT_DIR}"
if [[ "${EMBED}" == "1" ]]; then
  echo "Note: binaries were built with -tags embed_ytdlp (yt-dlp embedded)."
fi
