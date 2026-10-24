package server

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/innate/yt-dl/internal/vtt"
)

// vttLanguages tries each language in order (web-backend parity).
var vttLanguages = []string{"zh", "en", "ja", "ko", "zh-Hans", "zh-Hant"}

var cueCountRe = regexp.MustCompile(`\d{2}:\d{2}:\d{2}\s+\S`)

var videoIDRe = regexp.MustCompile(`(?:youtube\.com/(?:watch\?v=|embed/|shorts/)|youtu\.be/)([a-zA-Z0-9_-]{11})`)

// extractVideoID pulls the 11-char YouTube video id out of a URL.
func extractVideoID(rawURL string) *string {
	if m := videoIDRe.FindStringSubmatch(rawURL); m != nil {
		return &m[1]
	}
	return nil
}

// StartAnalysis records a pending VTT report and analyzes it in the
// background, mirroring the web backend's VttAnalyzer. Returns the report id.
func (s *Server) StartAnalysis(youtubeURL string) (string, error) {
	id := newUUID()
	videoID := extractVideoID(youtubeURL)
	report := VttReport{
		ID:         id,
		YoutubeURL: youtubeURL,
		VideoID:    videoID,
		Content:    "",
		CreatedAt:  nowISO(),
		Status:     "pending",
	}
	if err := s.db.CreateVttReport(report); err != nil {
		return "", err
	}
	s.hub.BroadcastType("vtt-report:status", map[string]any{"reportId": id, "status": "pending"})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = s.db.UpdateVttReport(id, VttReportUpdate{
					Status: strPtrVal("failed"),
					Error:  strPtrVal(fmt.Sprintf("analysis panic: %v", r)),
				})
			}
		}()
		s.runAnalysis(id, youtubeURL)
	}()
	return id, nil
}

// runAnalysis downloads subtitles, converts them to Markdown via the shared
// internal/vtt parser, and stores the result.
func (s *Server) runAnalysis(id, youtubeURL string) {
	vttDir := filepath.Join(s.defaultOutputDir, "vtt-temp", id)

	setFailed := func(err error) {
		msg := err.Error()
		_ = s.db.UpdateVttReport(id, VttReportUpdate{Status: strPtrVal("failed"), Error: &msg})
		s.hub.BroadcastType("vtt-report:status", map[string]any{"reportId": id, "status": "failed", "error": msg})
	}

	if err := os.MkdirAll(vttDir, 0o755); err != nil {
		setFailed(fmt.Errorf("create temp dir: %w", err))
		return
	}

	status := "processing"
	if err := s.db.UpdateVttReport(id, VttReportUpdate{Status: &status}); err != nil {
		setFailed(err)
		return
	}
	s.hub.BroadcastType("vtt-report:status", map[string]any{"reportId": id, "status": "processing"})

	var durationSec *float64
	if info, err := s.GetVideoInfo(youtubeURL); err == nil && info != nil {
		durationSec = info.Duration
		if info.Title != "" {
			_ = s.db.UpdateVttReport(id, VttReportUpdate{Title: &info.Title})
		}
	}

	language, vttPath, err := s.downloadVttFile(youtubeURL, vttDir)
	if err != nil {
		setFailed(err)
		s.cleanupVttDir(id, vttDir)
		return
	}

	markdown, cueCount, err := s.vttToMarkdown(vttPath)
	if err != nil {
		setFailed(err)
		s.cleanupVttDir(id, vttDir)
		return
	}

	err2 := s.db.UpdateVttReport(id, VttReportUpdate{
		Title:       nil,
		Language:    &language,
		Content:     &markdown,
		CueCount:    &cueCount,
		DurationSec: durationSec,
		VideoID:     extractVideoID(youtubeURL),
		Status:      strPtrVal("done"),
	})
	if err2 != nil {
		setFailed(err2)
		s.cleanupVttDir(id, vttDir)
		return
	}
	s.hub.BroadcastType("vtt-report:complete", map[string]any{"reportId": id})
	s.cleanupVttDir(id, vttDir)
}

// cleanupVttDir removes the temp dir once the report reached a terminal state.
func (s *Server) cleanupVttDir(id, dir string) {
	if report, err := s.db.GetVttReport(id); err == nil && report != nil {
		if report.Status == "done" || report.Status == "failed" {
			_ = os.RemoveAll(dir)
		}
	}
}

// downloadVttFile tries each preferred language until a subtitle file lands.
func (s *Server) downloadVttFile(youtubeURL, outputDir string) (string, string, error) {
	for _, lang := range vttLanguages {
		outTemplate := filepath.Join(outputDir, "%(id)s.%(lang)s.vtt")
		_, err := s.runYtDlp([]string{
			"--write-subs", "--write-auto-subs",
			"--sub-langs", lang,
			"--skip-download",
			"-o", outTemplate,
			youtubeURL,
		}, 60*time.Second)
		if err != nil {
			continue // try next language
		}
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".vtt") &&
				(strings.Contains(name, "."+lang+".") || strings.Contains(name, "."+lang+"-")) {
				return lang, filepath.Join(outputDir, name), nil
			}
		}
	}
	return "", "", fmt.Errorf("no subtitles found")
}

// vttToMarkdown parses a .vtt file with the shared parser and renders Markdown.
func (s *Server) vttToMarkdown(path string) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open vtt: %w", err)
	}
	defer f.Close()
	transcript, err := vtt.Parse(f)
	if err != nil {
		return "", 0, fmt.Errorf("parse vtt: %w", err)
	}
	markdown := transcript.ToMarkdown()
	if strings.TrimSpace(markdown) == "" {
		return "", 0, fmt.Errorf("no subtitles available for this video")
	}
	return markdown, len(cueCountRe.FindAllStringIndex(markdown, -1)), nil
}

// strPtrVal returns a pointer to the given string (small helper for updates).
func strPtrVal(s string) *string { return &s }
