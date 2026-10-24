package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// newUUID returns a random RFC 4122 v4 UUID (no external dependency).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on supported platforms; fall back to time.
		return fmt.Sprintf("uuid-error-%d", os.Getpid())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[v>>4], hex[v&0x0f])
	}
	return string(out)
}

// jsonEnvelope is the uniform API response shape: {success, data?, error?}.
type jsonEnvelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, jsonEnvelope{Success: true, Data: data})
}

func writeFail(w http.ResponseWriter, status int, errMsg string) {
	writeJSON(w, status, jsonEnvelope{Success: false, Error: errMsg})
}

// decodeJSON parses the request body into v; invalid JSON maps to 400 like
// the web backend's onError handler.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		writeFail(w, http.StatusBadRequest, err.Error())
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true // empty body: leave v zeroed
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeFail(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// corsMiddleware allows all origins (web-backend parity; bind addresses stay
// local unless the operator exposes the daemon, e.g. Docker).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SettingsGetResponse is the get-settings payload: fixed key set with the
// same JSON typing as the web backend (sub_langs array, concurrency number).
type SettingsGetResponse struct {
	YtDlpPath              *string `json:"yt_dlp_path"`
	DefaultOutputDir       *string `json:"default_output_dir"`
	DefaultQuality         *string `json:"default_quality"`
	DefaultFormat          *string `json:"default_format"`
	DefaultSubLangs        any     `json:"default_sub_langs"`
	Language               *string `json:"language"`
	MaxConcurrentDownloads any     `json:"max_concurrent_downloads"`
	AiProvider             *string `json:"ai_provider"`
	AiApiKey               *string `json:"ai_api_key"`
	AiModel                *string `json:"ai_model"`
	AgentCliKimiBin        *string `json:"agent_cli_kimi_bin"`
	AgentCliOtherBin       *string `json:"agent_cli_other_bin"`
}

var settingsKeys = []string{
	"yt_dlp_path", "default_output_dir", "default_quality", "default_format",
	"default_sub_langs", "language", "max_concurrent_downloads",
	"ai_provider", "ai_api_key", "ai_model",
	"agent_cli_kimi_bin", "agent_cli_other_bin",
}

// seedDefaults inserts missing default settings (web-backend parity).
func (s *Server) seedDefaults() error {
	ytdlpDefault, err := s.ytdlpPath()
	if err != nil {
		ytdlpDefault = "yt-dlp"
	}
	defaults := [][2]string{
		{"yt_dlp_path", ytdlpDefault},
		{"default_output_dir", s.defaultOutputDir},
		{"default_quality", "best"},
		{"default_format", "mp4"},
		{"default_sub_langs", `["en","zh"]`},
		{"language", "zh"},
		{"max_concurrent_downloads", "3"},
	}
	for _, kv := range defaults {
		existing, err := s.db.GetSetting(kv[0])
		if err != nil {
			return err
		}
		if existing == "" {
			if err := s.db.SetSetting(kv[0], kv[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// openPathLocally opens a path with the OS file manager / default app.
func openPathLocally(target string, reveal bool) error {
	var bin string
	var args []string
	switch {
	case reveal && isDarwin():
		bin, args = "open", []string{"-R", target}
	case isDarwin():
		bin, args = "open", []string{target}
	case reveal && isWindows():
		bin, args = "explorer", []string{"/select," + target}
	case isWindows():
		bin, args = "explorer", []string{target}
	default:
		bin, args = "xdg-open", []string{target}
	}
	cmd := spawnedCommand(bin, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err := <-done; err != nil {
		// explorer.exe exits 1 even on success
		if !(isWindows() && bin == "explorer") {
			return fmt.Errorf("%s exited with code %v", bin, err)
		}
	}
	return nil
}

// registerRoutes wires the full API surface (sprint-0 doc 01 §4).
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/ws", s.hub.ServeWS)

	mux.HandleFunc("POST /api/start-download", s.handleStartDownload)
	mux.HandleFunc("POST /api/cancel-download", s.handleCancelDownload)
	mux.HandleFunc("POST /api/retry-download", s.handleRetryDownload)
	mux.HandleFunc("POST /api/get-downloads", s.handleGetDownloads)
	mux.HandleFunc("GET /api/downloads", s.handleGetDownloads)
	mux.HandleFunc("POST /api/delete-download", s.handleDeleteDownload)

	mux.HandleFunc("POST /api/get-video-info", s.handleVideoInfo)
	mux.HandleFunc("POST /api/get-video-formats", s.handleVideoFormats)
	mux.HandleFunc("POST /api/get-playlist-info", s.handlePlaylistInfo)

	mux.HandleFunc("POST /api/get-settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/update-settings", s.handleUpdateSettings)
	mux.HandleFunc("POST /api/get-default-output-dir", s.handleGetDefaultOutputDir)

	// Desktop-only queue operations: clean JSON error, web UI shows a hint.
	mux.HandleFunc("POST /api/pause-download", func(w http.ResponseWriter, r *http.Request) {
		writeFail(w, http.StatusOK, "Pause/resume is not supported in web mode")
	})
	mux.HandleFunc("POST /api/resume-download", func(w http.ResponseWriter, r *http.Request) {
		writeFail(w, http.StatusOK, "Pause/resume is not supported in web mode")
	})

	mux.HandleFunc("POST /api/extract-audio", s.handleExtractAudio)
	mux.HandleFunc("POST /api/analyze-vtt", s.handleAnalyzeVtt)
	mux.HandleFunc("GET /api/vtt-reports", s.handleListVttReports)
	mux.HandleFunc("GET /api/vtt-report/{id}", s.handleGetVttReport)
	mux.HandleFunc("POST /api/delete-vtt-report", s.handleDeleteVttReport)

	mux.HandleFunc("POST /api/open-download-folder", s.handleOpenFolder(false))
	mux.HandleFunc("POST /api/reveal-in-folder", s.handleOpenFolder(true))
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"status":  "ok",
		"service": "vytdl-daemon",
		"version": Version,
	})
}

// handleStartDownload creates the record, persists the request for resume,
// and enqueues it. The frontend wraps args as {request: {...}} (Tauri mirror).
func (s *Server) handleStartDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Request *StartDownloadRequest `json:"request"`
		StartDownloadRequest
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	req := body.Request
	if req == nil {
		req = &body.StartDownloadRequest
	}
	if strings.TrimSpace(req.URL) == "" {
		writeFail(w, http.StatusBadRequest, "url is required")
		return
	}

	id := newUUID()
	outputDirSetting, _ := s.db.GetSetting("default_output_dir")
	outputDir := s.defaultOutputDir
	if outputDirSetting != "" {
		outputDir = outputDirSetting
	}
	if req.OutputDir != nil && strings.TrimSpace(*req.OutputDir) != "" {
		outputDir = *req.OutputDir
	}

	opts := req.ToDownloaderOptions(outputDir)
	optionsJSON, _ := json.Marshal(req)

	record := DownloadRecord{
		ID:              id,
		URL:             req.URL,
		Title:           blankToNil(deref(req.Title)),
		Status:          string(StatusPending),
		Progress:        0,
		Subtitles:       "[]",
		QueuePosition:   0,
		CollectionID:    blankToNil(deref(req.CollectionID)),
		CollectionTitle: blankToNil(deref(req.CollectionTitle)),
		OutputDir:       &outputDir,
		CreatedAt:       nowISO(),
		UpdatedAt:       nowISO(),
		Options:         strPtrVal(string(optionsJSON)),
	}
	if err := s.db.CreateDownload(record); err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.queue.Enqueue(id, opts)
	writeOK(w, id)
}

func blankToNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func (s *Server) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DownloadID string `json:"downloadId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	s.queue.Cancel(body.DownloadID)
	writeOK(w, nil)
}

func (s *Server) handleRetryDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	original, err := s.db.GetDownloadByID(body.ID)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if original == nil {
		writeFail(w, http.StatusNotFound, "Download not found")
		return
	}

	newID := newUUID()
	outputDirSetting, _ := s.db.GetSetting("default_output_dir")
	outputDir := s.defaultOutputDir
	if outputDirSetting != "" {
		outputDir = outputDirSetting
	}
	if original.OutputDir != nil && *original.OutputDir != "" {
		outputDir = *original.OutputDir
	}

	// Retry always re-runs as a single item (web-backend parity).
	req := StartDownloadRequest{URL: original.URL, OutputDir: &outputDir}
	if original.Options != nil && *original.Options != "" {
		var saved StartDownloadRequest
		if json.Unmarshal([]byte(*original.Options), &saved) == nil && saved.URL != "" {
			saved.URL = original.URL
			saved.OutputDir = &outputDir
			saved.IsPlaylist = false
			req = saved
		}
	}
	optionsJSON, _ := json.Marshal(req)

	record := DownloadRecord{
		ID:              newID,
		URL:             original.URL,
		Title:           original.Title, // keep the pre-fetched title
		Status:          string(StatusPending),
		Progress:        0,
		Subtitles:       "[]",
		QueuePosition:   0,
		CollectionID:    original.CollectionID,
		CollectionTitle: original.CollectionTitle,
		OutputDir:       &outputDir,
		CreatedAt:       nowISO(),
		UpdatedAt:       nowISO(),
		Options:         strPtrVal(string(optionsJSON)),
	}
	if err := s.db.CreateDownload(record); err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	opts := req.ToDownloaderOptions(outputDir)
	s.queue.Enqueue(newID, opts)
	writeOK(w, newID)
}

func (s *Server) handleGetDownloads(w http.ResponseWriter, r *http.Request) {
	downloads, err := s.db.GetAllDownloads()
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if downloads == nil {
		downloads = []DownloadRecord{}
	}
	writeOK(w, downloads)
}

func (s *Server) handleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	// Cancelling first mirrors the desktop behavior of not leaving orphans running.
	s.queue.Cancel(body.ID)
	if err := s.db.DeleteDownload(body.ID); err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, nil)
}

func (s *Server) handleVideoInfo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	info, err := s.GetVideoInfo(body.URL)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, info)
}

func (s *Server) handleVideoFormats(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	formats, err := s.GetVideoFormats(body.URL)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, formats)
}

func (s *Server) handlePlaylistInfo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	info, err := s.GetPlaylistInfo(body.URL)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, info)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	resp := SettingsGetResponse{
		DefaultSubLangs:        nil,
		MaxConcurrentDownloads: nil,
	}
	for _, key := range settingsKeys {
		value, err := s.db.GetSetting(key)
		if err != nil {
			writeFail(w, http.StatusInternalServerError, err.Error())
			return
		}
		switch key {
		case "yt_dlp_path":
			resp.YtDlpPath = blankToNil(value)
		case "default_output_dir":
			resp.DefaultOutputDir = blankToNil(value)
		case "default_quality":
			resp.DefaultQuality = blankToNil(value)
		case "default_format":
			resp.DefaultFormat = blankToNil(value)
		case "default_sub_langs":
			if value != "" {
				var langs []string
				if json.Unmarshal([]byte(value), &langs) == nil {
					resp.DefaultSubLangs = langs
				}
			}
		case "language":
			resp.Language = blankToNil(value)
		case "max_concurrent_downloads":
			if value != "" {
				if n, err := strconv.Atoi(value); err == nil {
					resp.MaxConcurrentDownloads = n
				}
			}
		case "ai_provider":
			resp.AiProvider = blankToNil(value)
		case "ai_api_key":
			resp.AiApiKey = blankToNil(value)
		case "ai_model":
			resp.AiModel = blankToNil(value)
		case "agent_cli_kimi_bin":
			resp.AgentCliKimiBin = blankToNil(value)
		case "agent_cli_other_bin":
			resp.AgentCliOtherBin = blankToNil(value)
		}
	}
	writeOK(w, resp)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Settings map[string]any `json:"settings"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	settings := body.Settings
	for key, value := range settings {
		if value == nil {
			continue
		}
		var stored string
		switch v := value.(type) {
		case string:
			stored = v
		case []any:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			stored = string(b)
		case bool:
			stored = strconv.FormatBool(v)
		case float64:
			stored = trimFloat(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			stored = string(b)
		}
		if err := s.db.SetSetting(key, stored); err != nil {
			writeFail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if v, ok := settings["max_concurrent_downloads"]; ok {
		switch n := v.(type) {
		case float64:
			s.queue.SetMaxConcurrent(int(n))
		case string:
			if parsed, err := strconv.Atoi(n); err == nil {
				s.queue.SetMaxConcurrent(parsed)
			}
		}
	}
	writeOK(w, nil)
}

func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}

func (s *Server) handleGetDefaultOutputDir(w http.ResponseWriter, r *http.Request) {
	dir, _ := s.db.GetSetting("default_output_dir")
	if dir == "" {
		dir = s.defaultOutputDir
	}
	writeOK(w, dir)
}

func (s *Server) handleExtractAudio(w http.ResponseWriter, r *http.Request) {
	var body struct {
		VideoPath   string `json:"video_path"`
		OutputDir   string `json:"output_dir"`
		AudioFormat string `json:"audio_format"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	format := body.AudioFormat
	if format == "" {
		format = "mp3"
	}
	audioPath, err := ExtractAudio(body.VideoPath, body.OutputDir, format)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"audio_path": audioPath})
}

func (s *Server) handleAnalyzeVtt(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.URL) == "" {
		writeFail(w, http.StatusBadRequest, "url is required")
		return
	}
	reportID, err := s.StartAnalysis(body.URL)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"reportId": reportID})
}

func (s *Server) handleListVttReports(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1, 1, 0)
	limit := queryInt(r, "limit", 20, 1, 100)
	lang := r.URL.Query().Get("lang")
	reports, total, err := s.db.ListVttReports(page, limit, lang)
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reports == nil {
		reports = []VttReport{}
	}
	writeOK(w, map[string]any{"reports": reports, "total": total})
}

func (s *Server) handleGetVttReport(w http.ResponseWriter, r *http.Request) {
	report, err := s.db.GetVttReport(r.PathValue("id"))
	if err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if report == nil {
		writeFail(w, http.StatusNotFound, "Report not found")
		return
	}
	writeOK(w, report)
}

func (s *Server) handleDeleteVttReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.db.DeleteVttReport(body.ID); err != nil {
		writeFail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, nil)
}

func (s *Server) handleOpenFolder(reveal bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.Path == "" {
			writeFail(w, http.StatusBadRequest, "path is required")
			return
		}
		verb := "open"
		if reveal {
			verb = "reveal"
		}
		if err := openPathLocally(body.Path, reveal); err != nil {
			// Expected only when the server runs in Docker (web-backend parity:
			// 200 with success:false so the UI shows a friendly message).
			writeFail(w, http.StatusOK,
				fmt.Sprintf("Cannot %s locally (expected only when the server runs in Docker): %v", verb, err))
			return
		}
		writeOK(w, nil)
	}
}

func queryInt(r *http.Request, key string, def, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if min > 0 && n < min {
		return min
	}
	if max > 0 && n > max {
		return max
	}
	return n
}

// staticHandler serves the exported Next.js frontend with SPA fallback to
// index.html (only when a static dir is configured).
func (s *Server) staticHandler() http.HandlerFunc {
	root := s.cfg.StaticDir
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeFail(w, http.StatusNotFound, "not found")
			return
		}
		clean := filepath.Clean("/" + r.URL.Path)
		full := filepath.Join(root, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, r, full)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	}
}
