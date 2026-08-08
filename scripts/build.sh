#!/usr/bin/env bash

set -euo pipefail

OUTPUT_DIR="${1:-dist}"
APP_NAME="${2:-vYtDL}"

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
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -trimpath -ldflags "-s -w" -o "${OUT_FILE}" "${PROJECT_DIR}"
done

echo "Build artifacts are in ${PROJECT_DIR}/${OUTPUT_DIR}"
