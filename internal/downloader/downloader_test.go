package downloader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/innate/yt-dl/internal/playliststate"
)

func TestBuildArgsIncludesExpectedFlags(t *testing.T) {
	t.Parallel()

	d := New(Options{
		Format:         "webm",
		Quality:        "720p",
		StartTime:      "00:00:05",
		EndTime:        "00:00:15",
		SubtitleLangs:  []string{"en", "zh"},
		WriteSubtitles: true,
		WriteAutoSubs:  true,
		OutputDir:      "/tmp/out",
		Retries:        "7",
		SocketTimeout:  "25",
		ForceIPv4:      true,
		ExtractorArgs:  "youtube:player_client=web,android",
	}, nil)

	args := d.buildArgs("https://example.com/watch?v=test", "/tmp/out")
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--download-sections *00:00:05-00:00:15") {
		t.Fatalf("expected time range in args, got %q", joined)
	}
	if !strings.Contains(joined, "--sub-langs en,zh") {
		t.Fatalf("expected subtitle languages in args, got %q", joined)
	}
	if !strings.Contains(joined, "--merge-output-format webm") {
		t.Fatalf("expected merge output format in args, got %q", joined)
	}
	if !strings.Contains(joined, "--no-playlist") {
		t.Fatalf("expected single-download mode in args, got %q", joined)
	}
	if !strings.Contains(joined, "bestvideo[height<=720]+bestaudio/best[height<=720]") {
		t.Fatalf("expected constrained format selector in args, got %q", joined)
	}
	if !strings.Contains(joined, "--retries 7 --extractor-retries 7") {
		t.Fatalf("expected retry flags in args, got %q", joined)
	}
	if !strings.Contains(joined, "--socket-timeout 25") {
		t.Fatalf("expected timeout flag in args, got %q", joined)
	}
	if !strings.Contains(joined, "--force-ipv4") {
		t.Fatalf("expected force-ipv4 flag in args, got %q", joined)
	}
	if !strings.Contains(joined, "--extractor-args youtube:player_client=web,android") {
		t.Fatalf("expected extractor args in args, got %q", joined)
	}
}

func TestDownloadPlaylistReturnsPerVideoResults(t *testing.T) {
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "yt-dlp")
	script := `#!/bin/sh
if [ "$1" = "--dump-single-json" ] && [ "$2" = "--flat-playlist" ]; then
  printf '%s\n' '{"title":"My Playlist","entries":[{"id":"vid1","webpage_url":"https://example.com/watch?v=vid1"},{"id":"vid2","url":"https://example.com/watch?v=vid2"}]}'
  exit 0
fi

last=""
for arg in "$@"; do
  last="$arg"
done

case "$last" in
  *vid1)
    printf '%s\n' '[download]   50.0% of 1.00MiB at  1.00MiB/s ETA 00:01'
    printf '%s\n' '{"id":"vid1","title":"Video One","ext":"mp4"}'
    exit 0
    ;;
  *vid2)
    printf '%s\n' '[download]  100.0% of 1.00MiB at  2.00MiB/s ETA 00:00'
    printf '%s\n' '{"id":"vid2","title":"Video Two","ext":"mp4"}'
    exit 0
    ;;
esac

printf '%s\n' "unexpected invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	d := New(Options{OutputDir: tempDir, Format: "mp4", IsPlaylist: true}, nil)

	results := d.DownloadPlaylist("https://example.com/playlist?id=abc")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	ids := []string{results[0].VideoID, results[1].VideoID}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"vid1", "vid2"}) {
		t.Fatalf("unexpected video ids: %#v", ids)
	}

	for _, result := range results {
		if !result.Success {
			t.Fatalf("expected successful result, got %#v", result)
		}
		if !strings.Contains(result.OutputDir, filepath.Join(tempDir, "My Playlist")) {
			t.Fatalf("expected playlist output dir, got %q", result.OutputDir)
		}
		if filepath.Ext(result.Filename) != ".mp4" {
			t.Fatalf("expected mp4 filename, got %q", result.Filename)
		}
	}
}

func TestNormalizeURLUnescapesShellEscapes(t *testing.T) {
	t.Parallel()

	got := normalizeURL(`"https://www.youtube.com/watch\?v\=EBWTRvjZ1dw\&list\=PL123"`)
	want := "https://www.youtube.com/watch?v=EBWTRvjZ1dw&list=PL123"
	if got != want {
		t.Fatalf("normalizeURL() = %q, want %q", got, want)
	}
}

func TestPlaylistEntryURLPrefersWebpageAndKeepsYouTubeFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry playlistEntry
		want  string
	}{
		{
			name:  "webpage_url wins",
			entry: playlistEntry{ID: "x", WebpageURL: "https://www.bilibili.com/video/BV1xx", IeKey: "BiliBili"},
			want:  "https://www.bilibili.com/video/BV1xx",
		},
		{
			name:  "absolute url wins",
			entry: playlistEntry{URL: "https://www.tiktok.com/@u/video/1", IeKey: "TikTok"},
			want:  "https://www.tiktok.com/@u/video/1",
		},
		{
			name:  "youtube id fallback",
			entry: playlistEntry{ID: "dQw4w9WgXcQ", IeKey: "Youtube"},
			want:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			name:  "empty ie_key still youtube fallback for legacy flat entries",
			entry: playlistEntry{ID: "abc123xyz01"},
			want:  "https://www.youtube.com/watch?v=abc123xyz01",
		},
		{
			name:  "non-youtube without url returns empty",
			entry: playlistEntry{ID: "BV1xx", IeKey: "BiliBili"},
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := playlistEntryURL(tc.entry)
			if got != tc.want {
				t.Fatalf("playlistEntryURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDownloadPlaylistResumesFromFailedEntries(t *testing.T) {
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "yt-dlp")
	script := `#!/bin/sh
state_dir="` + tempDir + `"
log_file="$state_dir/invocations.log"
printf '%s\n' "$*" >> "$log_file"

if [ "$1" = "--dump-single-json" ] && [ "$2" = "--flat-playlist" ]; then
  printf '%s\n' '{"title":"Resume Playlist","entries":[{"id":"vid1","title":"Video One","webpage_url":"https://example.com/watch?v=vid1"},{"id":"vid2","title":"Video Two","webpage_url":"https://example.com/watch?v=vid2"}]}'
  exit 0
fi

last=""
for arg in "$@"; do
  last="$arg"
done

case "$last" in
  *vid1)
    touch "Video One.mp4"
    printf '%s\n' '{"id":"vid1","title":"Video One","ext":"mp4"}'
    exit 0
    ;;
  *vid2)
    counter_file="$state_dir/vid2.count"
    count=0
    if [ -f "$counter_file" ]; then
      count="$(cat "$counter_file")"
    fi
    count=$((count + 1))
    printf '%s' "$count" > "$counter_file"
    if [ "$count" -eq 1 ]; then
      printf '%s\n' 'simulated failure' >&2
      exit 1
    fi
    touch "Video Two.mp4"
    printf '%s\n' '{"id":"vid2","title":"Video Two","ext":"mp4"}'
    exit 0
    ;;
esac

printf '%s\n' "unexpected invocation: $*" >&2
exit 1
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	d := New(Options{
		OutputDir:          tempDir,
		Format:             "mp4",
		IsPlaylist:         true,
		YTDLPBin:           fakeBin,
		ResetPlaylistState: false,
	}, nil)

	firstRun := d.DownloadPlaylist("https://example.com/playlist?id=resume")
	if len(firstRun) != 2 {
		t.Fatalf("expected 2 results on first run, got %d", len(firstRun))
	}
	if !firstRun[0].Success || firstRun[1].Success {
		t.Fatalf("expected success then failure on first run, got %#v", firstRun)
	}

	secondRun := d.DownloadPlaylist("https://example.com/playlist?id=resume")
	if len(secondRun) != 1 {
		t.Fatalf("expected only one retried result on second run, got %d", len(secondRun))
	}
	if !secondRun[0].Success || secondRun[0].VideoID != "vid2" {
		t.Fatalf("expected retried vid2 success on second run, got %#v", secondRun)
	}

	logData, err := os.ReadFile(filepath.Join(tempDir, "invocations.log"))
	if err != nil {
		t.Fatalf("read invocation log: %v", err)
	}
	logText := string(logData)
	if strings.Count(logText, "watch?v=vid1") != 1 {
		t.Fatalf("expected vid1 to run once, log=%q", logText)
	}
	if strings.Count(logText, "watch?v=vid2") != 2 {
		t.Fatalf("expected vid2 to run twice, log=%q", logText)
	}

	statePath := filepath.Join(tempDir, "Resume Playlist", ".playlist_state.json")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read playlist state: %v", err)
	}
	var state struct {
		Entries []playliststate.EntryState `json:"entries"`
	}
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("unmarshal playlist state: %v", err)
	}
	if len(state.Entries) != 2 {
		t.Fatalf("expected 2 state entries, got %#v", state.Entries)
	}
	for _, entry := range state.Entries {
		if entry.Status != playliststate.StatusSucceeded {
			t.Fatalf("expected final succeeded state, got %#v", state.Entries)
		}
	}
}
