package server

import (
	"os"
	"path/filepath"
	"testing"
)

// writeStubScript writes an executable shell script (test helper).
func writeStubScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
	return path
}

// stubYtDlpBody handles the four call shapes the daemon makes:
//   - --dump-json                  → video metadata
//   - --dump-single-json           → playlist metadata
//   - --sub-langs + --skip-download→ writes a small VTT file next to -o
//   - otherwise                    → download: progress lines + JSON + exit 0
//
// The URL decides behavior: containing "/slow" sleeps 2s, "/fail" exits 1.
func stubYtDlpBody() string {
	return `
URL=""
OUTARG=""
SKIP=""
SUBS_LANG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --dump-json) MODE="info" ;;
    --dump-single-json) MODE="playlist" ;;
    --sub-langs) SUBS_LANG="$2"; shift ;;
    --skip-download) SKIP="1" ;;
    -o) OUTARG="$2"; shift ;;
    http*) URL="$1" ;;
  esac
  shift
done

case "$URL" in
  *"/slow") sleep 2 ;;
  *"/fail") echo "simulated yt-dlp failure" >&2; exit 1 ;;
esac

if [ "$MODE" = "info" ]; then
  echo '{"id":"abc123","title":"Stub Video","ext":"mp4","duration":120.5,"thumbnail":"http://example.com/t.jpg","uploader":"Stub Channel","formats":[{"format_id":"137","quality_label":"1080p","resolution":"1920x1080","filesize":1048576},{"format_id":"140","quality":"medium","resolution":null,"filesize":null}]}'
  exit 0
fi

if [ "$MODE" = "playlist" ]; then
  echo '{"id":"pl1","title":"Stub Playlist","uploader":"Stub Channel","entries":[{"id":"abc123","title":"Entry One","duration":100,"url":"https://www.youtube.com/watch?v=abc123"},{"id":"def456","title":null,"duration":null,"thumbnails":[{"url":"http://example.com/last.jpg"}]}]}'
  exit 0
fi

if [ -n "$SKIP" ] && [ -n "$SUBS_LANG" ]; then
  DIR=$(dirname "$OUTARG")
  mkdir -p "$DIR"
  printf 'WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nhello stub\n\n00:00:04.000 --> 00:00:06.000\nsecond cue\n' > "$DIR/abc123.$SUBS_LANG.vtt"
  exit 0
fi

echo "[download] Destination: Stub Video.mp4"
echo "[download]  42.0% of ~10.00MiB at  1.00MiB/s ETA 00:05"
echo '{"id":"abc123","title":"Stub Video","ext":"mp4"}'
echo "[download] 100.0% of ~10.00MiB at  1.00MiB/s ETA 00:00"
echo "[info] Writing video subtitles to: Stub Video.en.vtt"
exit 0
`
}

// stubFfmpegBody exits 0 and reports the output path on stderr.
func stubFfmpegBody() string {
	return `echo "ffmpeg stub" >&2
exit 0
`
}
