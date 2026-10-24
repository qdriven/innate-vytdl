package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// testServer boots a daemon against a temp home with stubbed yt-dlp/ffmpeg.
type testServer struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	home   string
	outDir string
	ytdlp  string
	wsURL  string
}

func newTestServer(t *testing.T, ytdlpBody string) *testServer {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ytdlp := writeStubScript(t, binDir, "yt-dlp", ytdlpBody)
	writeStubScript(t, binDir, "ffmpeg", stubFfmpegBody())
	t.Setenv("FFMPEG_PATH", filepath.Join(binDir, "ffmpeg"))

	srv, err := New(Config{
		Host:      "127.0.0.1",
		Port:      0,
		StaticDir: "",
		HomeDir:   home,
		OutputDir: filepath.Join(home, "downloads"),
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	// Point every download + metadata call at the stub.
	if err := srv.db.SetSetting("yt_dlp_path", ytdlp); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		srv.Close()
	})
	return &testServer{t: t, srv: srv, ts: ts, home: home, outDir: srv.defaultOutputDir, ytdlp: ytdlp}
}

// post sends a JSON body and decodes the standard envelope.
func (ts *testServer) post(path string, body any) (int, map[string]any) {
	ts.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			ts.t.Fatal(err)
		}
	}
	resp, err := http.Post(ts.ts.URL+path, "application/json", &buf)
	if err != nil {
		ts.t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		ts.t.Fatalf("POST %s: decode: %v", path, err)
	}
	return resp.StatusCode, out
}

func (ts *testServer) get(path string) (int, map[string]any) {
	ts.t.Helper()
	resp, err := http.Get(ts.ts.URL + path)
	if err != nil {
		ts.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		ts.t.Fatalf("GET %s: decode: %v", path, err)
	}
	return resp.StatusCode, out
}

// waitForDownload polls until the record reaches status (10s cap).
func (ts *testServer) waitForDownload(id, status string) map[string]any {
	ts.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, out := ts.post("/api/get-downloads", nil)
		data, _ := out["data"].([]any)
		for _, row := range data {
			rec, _ := row.(map[string]any)
			if rec["id"] == id && rec["status"] == status {
				return rec
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	ts.t.Fatalf("download %s never reached status %q", id, status)
	return nil
}

func (ts *testServer) startDownload(url string) string {
	ts.t.Helper()
	code, out := ts.post("/api/start-download", map[string]any{
		"request": map[string]any{"url": url},
	})
	if code != 200 || out["success"] != true {
		ts.t.Fatalf("start-download: %d %v", code, out)
	}
	id, _ := out["data"].(string)
	if id == "" {
		ts.t.Fatalf("start-download returned no id: %v", out)
	}
	return id
}

// ── health ───────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.get("/api/health")
	if code != 200 || out["success"] != true {
		t.Fatalf("health: %d %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["status"] != "ok" {
		t.Fatalf("health data: %v", data)
	}
}

// ── full download flow (progress → complete) ────────────────────────────────

func TestDownloadFlowCompletes(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	id := ts.startDownload("https://example.com/watch?v=abc123")
	rec := ts.waitForDownload(id, "completed")

	if rec["progress"].(float64) != 100.0 {
		t.Fatalf("progress = %v", rec["progress"])
	}
	if rec["title"] != "Stub Video" {
		t.Fatalf("title = %v", rec["title"])
	}
	if rec["filename"] == "" || rec["filename"] == nil {
		t.Fatalf("filename missing: %v", rec)
	}
	// options JSON persisted for resume (internal column, not serialized)
	if row, err := ts.srv.db.GetDownloadByID(id); err != nil || row == nil || row.Options == nil || *row.Options == "" {
		t.Fatalf("options not persisted: row=%+v err=%v", row, err)
	}
	// output dir defaults to the daemon's downloads dir
	if rec["output_dir"] != ts.outDir {
		t.Fatalf("output_dir = %v, want %s", rec["output_dir"], ts.outDir)
	}
}

func TestStartDownloadBareBody(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	// No {request: ...} wrapper — bare body also accepted (web parity).
	code, out := ts.post("/api/start-download", map[string]any{"url": "https://example.com/v1"})
	if code != 200 || out["success"] != true {
		t.Fatalf("bare start: %d %v", code, out)
	}
	id, _ := out["data"].(string)
	ts.waitForDownload(id, "completed")
}

func TestStartDownloadRequiresURL(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/start-download", map[string]any{"request": map[string]any{}})
	if code != 400 || out["success"] != false {
		t.Fatalf("missing url: %d %v", code, out)
	}
}

func TestInvalidJSONMapsTo400(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	resp, err := http.Post(ts.ts.URL+"/api/start-download", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("invalid JSON status = %d", resp.StatusCode)
	}
}

func TestDownloadFailure(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	id := ts.startDownload("https://example.com/fail")
	rec := ts.waitForDownload(id, "failed")
	if rec["error"] == nil || rec["error"] == "" {
		t.Fatalf("error message missing: %v", rec)
	}
}

// ── cancel ───────────────────────────────────────────────────────────────────

func TestCancelActiveDownload(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	id := ts.startDownload("https://example.com/slow")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, out := ts.post("/api/get-downloads", nil)
		for _, row := range out["data"].([]any) {
			rec := row.(map[string]any)
			if rec["id"] == id && rec["status"] == "downloading" {
				goto started
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("download never started")
started:

	code, out := ts.post("/api/cancel-download", map[string]any{"downloadId": id})
	if code != 200 || out["success"] != true {
		t.Fatalf("cancel: %d %v", code, out)
	}
	ts.waitForDownload(id, "cancelled")
}

func TestCancelPendingDownload(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	ts.srv.queue.SetMaxConcurrent(1)

	// First item occupies the only slot ("/slow" runs 2s), second stays pending.
	first := ts.startDownload("https://example.com/slow")
	second := ts.startDownload("https://example.com/v2")

	code, out := ts.post("/api/cancel-download", map[string]any{"downloadId": second})
	if code != 200 || out["success"] != true {
		t.Fatalf("cancel pending: %d %v", code, out)
	}
	ts.waitForDownload(second, "cancelled")

	// Cancel the active one too so the test does not wait out the stub sleep.
	code, out = ts.post("/api/cancel-download", map[string]any{"downloadId": first})
	if code != 200 || out["success"] != true {
		t.Fatalf("cancel active: %d %v", code, out)
	}
	ts.waitForDownload(first, "cancelled")
}

// ── list / delete / retry ────────────────────────────────────────────────────

func TestListAndDeleteDownload(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	id := ts.startDownload("https://example.com/v1")
	ts.waitForDownload(id, "completed")

	code, out := ts.post("/api/get-downloads", nil)
	data, _ := out["data"].([]any)
	if code != 200 || len(data) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}

	// GET alias
	code, out = ts.get("/api/downloads")
	if code != 200 || out["success"] != true {
		t.Fatalf("GET /api/downloads: %d %v", code, out)
	}

	code, out = ts.post("/api/delete-download", map[string]any{"id": id})
	if code != 200 || out["success"] != true {
		t.Fatalf("delete: %d %v", code, out)
	}
	_, out = ts.post("/api/get-downloads", nil)
	if data, _ = out["data"].([]any); len(data) != 0 {
		t.Fatalf("list after delete: %v", out)
	}
}

func TestRetryDownload(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	// Seed a failed record with saved options (as if a real run failed).
	db := ts.srv.db
	failID := newUUID()
	optsJSON := `{"url":"https://example.com/v1","is_playlist":false}`
	title := "Preset Title"
	if err := db.CreateDownload(DownloadRecord{
		ID: failID, URL: "https://example.com/v1", Title: &title,
		Status: "failed", Subtitles: "[]", CreatedAt: nowISO(), UpdatedAt: nowISO(),
		Options: &optsJSON,
	}); err != nil {
		t.Fatal(err)
	}

	code, out := ts.post("/api/retry-download", map[string]any{"id": failID})
	if code != 200 || out["success"] != true {
		t.Fatalf("retry: %d %v", code, out)
	}
	newID, _ := out["data"].(string)
	if newID == "" || newID == failID {
		t.Fatalf("retry id = %q", newID)
	}
	rec := ts.waitForDownload(newID, "completed")
	// pre-fetched title carried over until the stub overwrites it
	if rec["title"] == "" {
		t.Fatalf("retry lost title: %v", rec)
	}

	// unknown id → 404
	code, _ = ts.post("/api/retry-download", map[string]any{"id": "nope"})
	if code != 404 {
		t.Fatalf("retry unknown status = %d", code)
	}
}

// ── pause/resume (unsupported, clean JSON error) ─────────────────────────────

func TestPauseResumeUnsupported(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	for _, path := range []string{"/api/pause-download", "/api/resume-download"} {
		code, out := ts.post(path, map[string]any{"id": "x"})
		if code != 200 || out["success"] != false {
			t.Fatalf("%s: %d %v", path, code, out)
		}
		if !strings.Contains(fmt.Sprint(out["error"]), "not supported") {
			t.Fatalf("%s error = %v", path, out["error"])
		}
	}
}

// ── metadata endpoints ───────────────────────────────────────────────────────

func TestGetVideoInfo(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/get-video-info", map[string]any{"url": "https://example.com/watch?v=abc123"})
	if code != 200 || out["success"] != true {
		t.Fatalf("video-info: %d %v", code, out)
	}
	info := out["data"].(map[string]any)
	if info["title"] != "Stub Video" || info["id"] != "abc123" {
		t.Fatalf("info = %v", info)
	}
	formats, _ := info["formats"].([]any)
	if len(formats) != 2 {
		t.Fatalf("formats = %v", formats)
	}
	first := formats[0].(map[string]any)
	if first["quality"] != "1080p" || first["format_id"] != "137" {
		t.Fatalf("format[0] = %v", first)
	}

	code, out = ts.post("/api/get-video-formats", map[string]any{"url": "https://example.com/watch?v=abc123"})
	formats, _ = out["data"].([]any)
	if code != 200 || len(formats) != 2 {
		t.Fatalf("video-formats: %d %v", code, out)
	}
}

func TestGetPlaylistInfo(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/get-playlist-info", map[string]any{"url": "https://example.com/playlist?list=pl1"})
	if code != 200 || out["success"] != true {
		t.Fatalf("playlist-info: %d %v", code, out)
	}
	info := out["data"].(map[string]any)
	if info["title"] != "Stub Playlist" {
		t.Fatalf("playlist = %v", info)
	}
	entries, _ := info["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	second := entries[1].(map[string]any)
	// null title passes through empty (frontend labels it unavailable)
	if second["title"] != "" {
		t.Fatalf("null title handling: %v", second)
	}
	if second["thumbnail"] != "http://example.com/last.jpg" {
		t.Fatalf("thumbnails fallback: %v", second)
	}
}

// ── settings ─────────────────────────────────────────────────────────────────

func TestSettingsDefaultsAndRoundtrip(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())

	code, out := ts.post("/api/get-settings", nil)
	if code != 200 || out["success"] != true {
		t.Fatalf("get-settings: %d %v", code, out)
	}
	s := out["data"].(map[string]any)
	if s["default_quality"] != "best" || s["default_format"] != "mp4" || s["language"] != "zh" {
		t.Fatalf("defaults = %v", s)
	}
	// typed payloads: sub_langs array + concurrency number
	if langs, ok := s["default_sub_langs"].([]any); !ok || len(langs) != 2 {
		t.Fatalf("sub_langs = %v (%T)", s["default_sub_langs"], s["default_sub_langs"])
	}
	if n, ok := s["max_concurrent_downloads"].(float64); !ok || int(n) != 3 {
		t.Fatalf("max_concurrent = %v (%T)", s["max_concurrent_downloads"], s["max_concurrent_downloads"])
	}

	code, out = ts.post("/api/update-settings", map[string]any{
		"settings": map[string]any{
			"max_concurrent_downloads": 5,
			"default_sub_langs":        []string{"en", "ja"},
			"language":                 "en",
		},
	})
	if code != 200 || out["success"] != true {
		t.Fatalf("update-settings: %d %v", code, out)
	}
	_, out = ts.post("/api/get-settings", nil)
	s = out["data"].(map[string]any)
	if n, _ := s["max_concurrent_downloads"].(float64); int(n) != 5 {
		t.Fatalf("max_concurrent after update = %v", s["max_concurrent_downloads"])
	}
	if ts.srv.queue.MaxConcurrent() != 5 {
		t.Fatalf("queue concurrency = %d", ts.srv.queue.MaxConcurrent())
	}
	if langs, _ := s["default_sub_langs"].([]any); len(langs) != 2 || langs[0] != "en" || langs[1] != "ja" {
		t.Fatalf("sub_langs after update = %v", s["default_sub_langs"])
	}
	if s["language"] != "en" {
		t.Fatalf("language after update = %v", s["language"])
	}
}

func TestGetDefaultOutputDir(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/get-default-output-dir", nil)
	if code != 200 || out["data"] != ts.outDir {
		t.Fatalf("default output dir: %d %v", code, out)
	}
}

// ── extract audio ────────────────────────────────────────────────────────────

func TestExtractAudio(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	video := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := ts.post("/api/extract-audio", map[string]any{
		"video_path": video, "output_dir": "", "audio_format": "mp3",
	})
	if code != 200 || out["success"] != true {
		t.Fatalf("extract-audio: %d %v", code, out)
	}
	data := out["data"].(map[string]any)
	if !strings.HasSuffix(data["audio_path"].(string), "video.mp3") {
		t.Fatalf("audio_path = %v", data["audio_path"])
	}

	// missing file → error envelope with 500 (web onError parity)
	code, out = ts.post("/api/extract-audio", map[string]any{
		"video_path": "/nonexistent.mp4", "audio_format": "mp3",
	})
	if code != 500 || out["success"] != false {
		t.Fatalf("extract-audio missing file: %d %v", code, out)
	}
}

// ── open folder ──────────────────────────────────────────────────────────────

func TestOpenFolderValidation(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/open-download-folder", map[string]any{})
	if code != 400 || out["success"] != false {
		t.Fatalf("open without path: %d %v", code, out)
	}
	code, out = ts.post("/api/reveal-in-folder", map[string]any{})
	if code != 400 || out["success"] != false {
		t.Fatalf("reveal without path: %d %v", code, out)
	}
	// With a path the OS opener may or may not succeed; both paths must
	// answer HTTP 200 with the documented envelope.
	code, out = ts.post("/api/open-download-folder", map[string]any{"path": t.TempDir()})
	if code != 200 {
		t.Fatalf("open with path: %d %v", code, out)
	}
}

// ── VTT analysis ─────────────────────────────────────────────────────────────

func TestVttAnalysisLifecycle(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	// 11-char video id so extraction matches YouTube's id shape.
	code, out := ts.post("/api/analyze-vtt", map[string]any{"url": "https://www.youtube.com/watch?v=abc12345678"})
	if code != 200 || out["success"] != true {
		t.Fatalf("analyze-vtt: %d %v", code, out)
	}
	reportID := out["data"].(map[string]any)["reportId"].(string)

	deadline := time.Now().Add(10 * time.Second)
	var report map[string]any
	for time.Now().Before(deadline) {
		code, out := ts.get("/api/vtt-report/" + reportID)
		if code == 200 {
			report = out["data"].(map[string]any)
			if report["status"] == "done" || report["status"] == "failed" {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if report == nil || report["status"] != "done" {
		t.Fatalf("report not done: %v", report)
	}
	content, _ := report["content"].(string)
	if !strings.Contains(content, "hello stub") {
		t.Fatalf("markdown content = %q", content)
	}
	if lang, _ := report["language"].(string); lang != "zh" {
		t.Fatalf("language = %v", report["language"])
	}
	if videoID, _ := report["video_id"].(string); videoID != "abc12345678" {
		t.Fatalf("video_id = %v", report["video_id"])
	}

	// list
	code, out = ts.get("/api/vtt-reports?page=1&limit=20")
	if code != 200 {
		t.Fatalf("list reports: %d %v", code, out)
	}
	list := out["data"].(map[string]any)
	if total, _ := list["total"].(float64); int(total) != 1 {
		t.Fatalf("total = %v", list["total"])
	}
	if reports, _ := list["reports"].([]any); len(reports) != 1 {
		t.Fatalf("reports = %v", list["reports"])
	}

	// delete
	code, out = ts.post("/api/delete-vtt-report", map[string]any{"id": reportID})
	if code != 200 || out["success"] != true {
		t.Fatalf("delete report: %d %v", code, out)
	}
	code, _ = ts.get("/api/vtt-report/" + reportID)
	if code != 404 {
		t.Fatalf("deleted report fetch = %d", code)
	}
}

func TestAnalyzeVttRequiresURL(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	code, out := ts.post("/api/analyze-vtt", map[string]any{})
	if code != 400 || out["success"] != false {
		t.Fatalf("analyze without url: %d %v", code, out)
	}
}

// ── WebSocket broadcast ──────────────────────────────────────────────────────

func TestWebSocketBroadcastsEvents(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())

	wsURL := "ws" + strings.TrimPrefix(ts.ts.URL, "http") + "/api/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.CloseNow()

	ts.startDownload("https://example.com/watch?v=abc123")

	events := map[string]bool{}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			continue
		}
		var msg struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if strings.HasPrefix(msg.Event, "download:status:") {
			var status string
			if json.Unmarshal(msg.Data, &status) == nil {
				events["status:"+status] = true
			}
		}
		if strings.HasPrefix(msg.Event, "download:progress:") {
			events["progress"] = true
		}
		if msg.Event == "queue:update" {
			events["queue:update"] = true
		}
		if strings.HasPrefix(msg.Event, "download:complete:") {
			events["complete"] = true
		}
		if events["status:completed"] && events["progress"] && events["queue:update"] {
			break
		}
	}
	if !events["status:completed"] || !events["progress"] || !events["queue:update"] {
		t.Fatalf("missing ws events: %v", events)
	}
}

// ── static hosting + SPA fallback ────────────────────────────────────────────

func TestStaticHostingWithSPAFallback(t *testing.T) {
	home := t.TempDir()
	staticDir := filepath.Join(home, "out")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Config{Host: "127.0.0.1", HomeDir: home, StaticDir: staticDir})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	defer srv.Close()

	// asset
	resp, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || body.String() != "console.log(1)" {
		t.Fatalf("asset fetch: %d %q", resp.StatusCode, body.String())
	}

	// SPA fallback for client-side routes
	resp, err = http.Get(ts.URL + "/settings/page")
	if err != nil {
		t.Fatal(err)
	}
	body.Reset()
	_, _ = body.ReadFrom(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(body.String(), "spa") {
		t.Fatalf("SPA fallback: %d %q", resp.StatusCode, body.String())
	}
}

// ── resume on startup ────────────────────────────────────────────────────────

func TestResumeOnStartup(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ytdlp := writeStubScript(t, binDir, "yt-dlp", stubYtDlpBody())

	// Seed the database with in-flight records from a "previous run".
	db, err := OpenDatabase(filepath.Join(home, "vytdl.db"))
	if err != nil {
		t.Fatal(err)
	}
	stuck := DownloadRecord{
		ID: newUUID(), URL: "https://example.com/watch?v=abc123",
		Status: "downloading", Subtitles: "[]",
		CreatedAt: nowISO(), UpdatedAt: nowISO(),
		Options: strPtrVal(`{"url":"https://example.com/watch?v=abc123","is_playlist":false}`),
	}
	pendingRow := DownloadRecord{
		ID: newUUID(), URL: "https://example.com/watch?v=abc123",
		Status: "pending", Subtitles: "[]",
		CreatedAt: nowISO(), UpdatedAt: nowISO(),
		Options: strPtrVal(`{"url":"https://example.com/watch?v=abc123","is_playlist":false}`),
	}
	noOpts := DownloadRecord{
		ID: newUUID(), URL: "https://example.com/watch?v=abc123",
		Status: "downloading", Subtitles: "[]",
		CreatedAt: nowISO(), UpdatedAt: nowISO(),
	}
	for _, r := range []DownloadRecord{stuck, pendingRow, noOpts} {
		if err := db.CreateDownload(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SetSetting("yt_dlp_path", ytdlp); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Config{Host: "127.0.0.1", HomeDir: home})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	defer srv.Close()

	tst := &testServer{t: t, srv: srv, ts: ts, home: home, outDir: srv.defaultOutputDir}
	tst.waitForDownload(stuck.ID, "completed")
	tst.waitForDownload(pendingRow.ID, "completed")

	// record without options is failed with a clear message
	code, out := tst.post("/api/get-downloads", nil)
	if code != 200 {
		t.Fatalf("list: %d", code)
	}
	for _, row := range out["data"].([]any) {
		rec := row.(map[string]any)
		if rec["id"] == noOpts.ID {
			if rec["status"] != "failed" {
				t.Fatalf("no-options record status = %v", rec["status"])
			}
			if !strings.Contains(fmt.Sprint(rec["error"]), "no saved download options") {
				t.Fatalf("no-options error = %v", rec["error"])
			}
		}
	}
}

// ── concurrency cap ──────────────────────────────────────────────────────────

func TestMaxConcurrentClamping(t *testing.T) {
	ts := newTestServer(t, stubYtDlpBody())
	ts.srv.queue.SetMaxConcurrent(99)
	if got := ts.srv.queue.MaxConcurrent(); got != 10 {
		t.Fatalf("clamp high: %d", got)
	}
	ts.srv.queue.SetMaxConcurrent(0)
	if got := ts.srv.queue.MaxConcurrent(); got != 1 {
		t.Fatalf("clamp low: %d", got)
	}
}
