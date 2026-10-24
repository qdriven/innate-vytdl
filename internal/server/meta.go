package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/innate/yt-dl/internal/ytdlpbin"
)

// VideoFormat is one selectable format in video-info payloads.
type VideoFormat struct {
	FormatID   string  `json:"format_id"`
	Quality    string  `json:"quality"`
	Resolution *string `json:"resolution"`
	Filesize   *int64  `json:"filesize"`
}

// VideoInfo is the get-video-info response payload.
type VideoInfo struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Duration  *float64      `json:"duration"`
	Thumbnail *string       `json:"thumbnail"`
	Uploader  *string       `json:"uploader"`
	Formats   []VideoFormat `json:"formats"`
}

// PlaylistEntry is one flat entry of a playlist-info payload.
type PlaylistEntry struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Duration   *float64 `json:"duration"`
	Thumbnail  *string  `json:"thumbnail"`
	WebpageURL *string  `json:"webpage_url,omitempty"`
}

// PlaylistInfo is the get-playlist-info response payload.
type PlaylistInfo struct {
	ID       *string         `json:"id"`
	Title    string          `json:"title"`
	Uploader *string         `json:"uploader"`
	Entries  []PlaylistEntry `json:"entries"`
}

// runYtDlp runs yt-dlp with the given args and a hard timeout, returning stdout.
// Non-standard *_proxy environment variables are stripped (same fix as the
// Rust backend and the previous Node backend).
func (s *Server) runYtDlp(args []string, timeout time.Duration) (string, error) {
	bin, err := s.ytdlpPath()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = filteredEnv()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("yt-dlp command timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return string(out), nil
}

// filteredEnv keeps only well-known proxy variables.
func filteredEnv() []string {
	allowed := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
		"http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
	}
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasSuffix(strings.ToLower(key), "_proxy") && !allowed[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// ytdlpPath resolves the yt-dlp binary (setting override → PATH → embedded → cache).
func (s *Server) ytdlpPath() (string, error) {
	if v, err := s.db.GetSetting("yt_dlp_path"); err == nil && strings.TrimSpace(v) != "" {
		if path, err := ytdlpbin.Resolve(v); err == nil {
			return path, nil
		}
	}
	return ytdlpbin.Resolve("")
}

var twitterHosts = map[string]bool{
	"twitter.com": true, "x.com": true, "vxtwitter.com": true,
	"fxtwitter.com": true, "nitter.net": true,
}

// isTwitterURL reports whether the URL points at a Twitter/X host.
func isTwitterURL(raw string) bool {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "mobile.")
	return twitterHosts[host] || strings.HasSuffix(host, ".twitter.com")
}

// siteExtractorArgs mirrors the web backend: auto twitter syndication API arg,
// then user-specified extractor args.
func siteExtractorArgs(siteURL, existing string) []string {
	var args []string
	hasTwitterAPI := strings.Contains(strings.ToLower(existing), "twitter:api=")
	if isTwitterURL(siteURL) && !hasTwitterAPI {
		args = append(args, "--extractor-args", "twitter:api=syndication")
	}
	if v := strings.TrimSpace(existing); v != "" {
		args = append(args, "--extractor-args", v)
	}
	return args
}

// lastJSONObject grabs the final line of yt-dlp JSON output (web parity).
func lastJSONObject(out string) (json.RawMessage, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			return json.RawMessage(line), nil
		}
	}
	return nil, fmt.Errorf("no JSON in yt-dlp output")
}

func jsonFloat(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	}
	return nil
}

func jsonString(v any) *string {
	switch s := v.(type) {
	case string:
		if s == "" {
			return nil
		}
		return &s
	}
	return nil
}

// GetVideoInfo implements POST /api/get-video-info. The 40s timeout stays
// under the frontend's 45s extraction timeout.
func (s *Server) GetVideoInfo(siteURL string) (*VideoInfo, error) {
	args := []string{
		"--dump-json", "--no-playlist", "--skip-download", "--no-warnings",
		"--socket-timeout", "10",
	}
	args = append(args, siteExtractorArgs(siteURL, "")...)
	args = append(args, siteURL)
	out, err := s.runYtDlp(args, 40*time.Second)
	if err != nil {
		return nil, err
	}
	raw, err := lastJSONObject(out)
	if err != nil {
		return nil, err
	}
	var data struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Duration  any     `json:"duration"`
		Thumbnail *string `json:"thumbnail"`
		Uploader  *string `json:"uploader"`
		Formats   []struct {
			FormatID     string  `json:"format_id"`
			QualityLabel *string `json:"quality_label"`
			Quality      any     `json:"quality"`
			Resolution   *string `json:"resolution"`
			Filesize     *int64  `json:"filesize"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}
	info := &VideoInfo{
		ID:        data.ID,
		Title:     orFallback(data.Title, "Unknown"),
		Duration:  jsonFloat(data.Duration),
		Thumbnail: data.Thumbnail,
		Uploader:  data.Uploader,
		Formats:   []VideoFormat{},
	}
	for _, f := range data.Formats {
		quality := ""
		switch q := f.Quality.(type) {
		case string:
			quality = q
		case float64:
			quality = fmt.Sprintf("%g", q)
		}
		if f.QualityLabel != nil && *f.QualityLabel != "" {
			quality = *f.QualityLabel
		}
		if quality == "" {
			quality = f.FormatID
		}
		info.Formats = append(info.Formats, VideoFormat{
			FormatID:   f.FormatID,
			Quality:    quality,
			Resolution: f.Resolution,
			Filesize:   f.Filesize,
		})
	}
	return info, nil
}

// GetVideoFormats implements POST /api/get-video-formats.
func (s *Server) GetVideoFormats(siteURL string) ([]VideoFormat, error) {
	info, err := s.GetVideoInfo(siteURL)
	if err != nil {
		return nil, err
	}
	return info.Formats, nil
}

// GetPlaylistInfo implements POST /api/get-playlist-info. The 55s timeout
// stays under the frontend's 60s collection-preview timeout.
func (s *Server) GetPlaylistInfo(siteURL string) (*PlaylistInfo, error) {
	args := []string{
		"--dump-single-json", "--flat-playlist", "--skip-download", "--no-warnings",
		"--socket-timeout", "10",
	}
	args = append(args, siteExtractorArgs(siteURL, "")...)
	args = append(args, siteURL)
	out, err := s.runYtDlp(args, 55*time.Second)
	if err != nil {
		return nil, err
	}
	raw, err := lastJSONObject(out)
	if err != nil {
		return nil, err
	}
	var meta struct {
		ID       *string `json:"id"`
		Title    *string `json:"title"`
		Uploader *string `json:"uploader"`
		Entries  []struct {
			ID         string  `json:"id"`
			Title      *string `json:"title"`
			Duration   any     `json:"duration"`
			Thumbnail  *string `json:"thumbnail"`
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
			URL        *string `json:"url"`
			WebpageURL *string `json:"webpage_url"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}
	info := &PlaylistInfo{
		ID:       meta.ID,
		Title:    orFallback(deref(meta.Title), "Playlist"),
		Uploader: meta.Uploader,
		Entries:  []PlaylistEntry{},
	}
	for _, e := range meta.Entries {
		entry := PlaylistEntry{
			ID:        e.ID,
			Title:     deref(e.Title), // private/deleted videos: empty title → frontend shows unavailable
			Duration:  jsonFloat(e.Duration),
			Thumbnail: e.Thumbnail,
		}
		if entry.Thumbnail == nil && len(e.Thumbnails) > 0 {
			u := e.Thumbnails[len(e.Thumbnails)-1].URL
			if u != "" {
				entry.Thumbnail = &u
			}
		}
		webpage := deref(e.WebpageURL)
		if isBlank(webpage) {
			webpage = deref(e.URL)
		}
		if !isBlank(webpage) {
			entry.WebpageURL = &webpage
		}
		info.Entries = append(info.Entries, entry)
	}
	return info, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

func orFallback(s, def string) string {
	if isBlank(s) {
		return def
	}
	return s
}
