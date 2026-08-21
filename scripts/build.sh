#!/usr/bin/env bash
# Cross-compile vYtDL for macOS / Linux / Windows.
#
# Usage:
#   ./scripts/build.sh [outdir] [appname]
#   EMBED=1 ./scripts/build.sh
#   ONLY=windows ./scripts/build.sh          # windows amd64 + arm64
#   ONLY=windows/amd64 ./scripts/build.sh    # single target
#   ONLY=darwin,linux ./scripts/build.sh

set -euo pipefail

# Ignore parent go.work when this tree is checked out inside a monorepo.
export GOWORK="${GOWORK:-off}"

OUTPUT_DIR="${1:-dist}"
APP_NAME="${2:-vYtDL}"
EMBED="${EMBED:-0}"
ONLY="${ONLY:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

mkdir -p "${PROJECT_DIR}/${OUTPUT_DIR}"

ALL_TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

should_build() {
  local goos="$1" goarch="$2"
  [[ -z "${ONLY}" ]] && return 0

  local token
  IFS=',' read -ra tokens <<<"${ONLY}"
  for token in "${tokens[@]}"; do
    # trim whitespace
    token="${token#"${token%%[![:space:]]*}"}"
    token="${token%"${token##*[![:space:]]}"}"
    token="$(printf '%s' "${token}" | tr '[:upper:]' '[:lower:]')"
    case "${token}" in
      "${goos}"|"${goos}/${goarch}"|"${goos}-${goarch}"|"${goos}_${goarch}")
        return 0
        ;;
    esac
  done
  return 1
}

built=0
for target in "${ALL_TARGETS[@]}"; do
  read -r GOOS GOARCH <<<"${target}"
  if ! should_build "${GOOS}" "${GOARCH}"; then
    continue
  fi

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
  built=$((built + 1))
done

if [[ "${built}" -eq 0 ]]; then
  echo "No targets matched ONLY=${ONLY}" >&2
  echo "Examples: ONLY=windows  ONLY=windows/amd64  ONLY=darwin,linux" >&2
  exit 1
fi

echo "Build artifacts are in ${PROJECT_DIR}/${OUTPUT_DIR} (${built} target(s))"
if [[ "${EMBED}" == "1" ]]; then
  echo "Note: binaries were built with -tags embed_ytdlp (yt-dlp embedded)."
fi
