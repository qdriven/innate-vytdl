package downloader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/innate/yt-dl/internal/playliststate"
	"github.com/innate/yt-dl/internal/ytdlpbin"
)

// ProgressUpdate is sent over a channel to report download progress.
type ProgressUpdate struct {
	Key     string
	VideoID string
	Title   string
	Percent float64
	Speed   string
	ETA     string
	Status  string // "downloading", "merging", "done", "error"
	Error   string
}

// VideoInfo holds metadata extracted from yt-dlp --dump-json.
type VideoInfo struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Ext           string `json:"ext"`
	PlaylistID    string `json:"playlist_id"`
	PlaylistTitle string `json:"playlist_title"`
	Filename      string // resolved output filename
}

// Downloader wraps yt-dlp for video and playlist downloads.
type Downloader struct {
	opts     Options
	progress chan<- ProgressUpdate
}

type playlistEntry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	WebpageURL string `json:"webpage_url"`
	IeKey      string `json:"ie_key"`
}

type playlistMetadata struct {
	Title   string          `json:"title"`
	Entries []playlistEntry `json:"entries"`
}

// New creates a new Downloader. progress receives live updates (may be nil).
func New(opts Options, progress chan<- ProgressUpdate) *Downloader {
	return &Downloader{opts: opts, progress: progress}
}

// ytdlpBin returns the yt-dlp binary path (PATH, embed, or auto-download cache).
func ytdlpBin() (string, error) {
	return ytdlpbin.Resolve("")
}

// buildArgs constructs yt-dlp arguments from options.
func (d *Downloader) buildArgs(url, outDir string) []string {
	o := d.opts
	args := []string{}
	container := strings.TrimSpace(o.Format)

	// Format selection
	if o.Quality != "" && o.Quality != "bestvideo+bestaudio" {
		// quality like "720", "1080" → select best video up to that height
		height := strings.TrimSuffix(o.Quality, "p")
		videoSel := fmt.Sprintf("bestvideo[height<=%s]", height)
		bestSel := fmt.Sprintf("best[height<=%s]", height)
		fmtStr := fmt.Sprintf("%s+bestaudio/%s", videoSel, bestSel)
		args = append(args, "-f", fmtStr)
	} else {
		args = append(args, "-f", "bestvideo+bestaudio/best")
	}

	// Merge format
	if container != "" {
		args = append(args, "--merge-output-format", container)
	}

	// Output template
	outTemplate := filepath.Join(outDir, "%(title)s.%(ext)s")
	args = append(args, "-o", outTemplate)

	// Subtitles
	if o.WriteSubtitles {
		args = append(args, "--write-subs")
		if o.WriteAutoSubs {
			args = append(args, "--write-auto-subs")
		}
		if len(o.SubtitleLangs) > 0 {
			args = append(args, "--sub-langs", strings.Join(o.SubtitleLangs, ","))
		}
	}

	// Time range (requires ffmpeg)
	if o.StartTime != "" || o.EndTime != "" {
		section := ""
		if o.StartTime != "" {
			section += "*" + o.StartTime + "-"
		} else {
			section += "*0-"
		}
		if o.EndTime != "" {
			section += o.EndTime
		} else {
			section += "inf"
		}
		args = append(args, "--download-sections", section, "--force-keyframes-at-cuts")
	}

	// Playlist handling
	if !o.IsPlaylist {
		args = append(args, "--no-playlist")
	}

	if retries := strings.TrimSpace(o.Retries); retries != "" {
		args = append(args, "--retries", retries, "--extractor-retries", retries)
	}
	if timeout := strings.TrimSpace(o.SocketTimeout); timeout != "" {
		args = append(args, "--socket-timeout", timeout)
	}
	if o.ForceIPv4 {
		args = append(args, "--force-ipv4")
	}
	if proxy := strings.TrimSpace(o.Proxy); proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	if cookies := strings.TrimSpace(o.CookiesFile); cookies != "" {
		args = append(args, "--cookies", cookies)
	}
	if browser := strings.TrimSpace(o.CookiesFromBrowser); browser != "" {
		args = append(args, "--cookies-from-browser", browser)
	}
	if ua := strings.TrimSpace(o.UserAgent); ua != "" {
		args = append(args, "--user-agent", ua)
	}
	if extractorArgs := strings.TrimSpace(o.ExtractorArgs); extractorArgs != "" {
		args = append(args, "--extractor-args", extractorArgs)
	}

	// Progress output in newline-delimited form for parsing
	args = append(args, "--newline", "--progress")

	// JSON metadata for post-processing
	args = append(args, "--print-json")

	args = append(args, url)
	return args
}

// progressRe matches yt-dlp progress lines like:
// [download]  42.3% of ~100.00MiB at  2.00MiB/s ETA 00:30
var progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%.*?at\s+(\S+)\s+ETA\s+(\S+)`)

// DownloadResult holds the outcome of a single video download attempt.
type DownloadResult struct {
	VideoID    string
	Title      string
	URL        string
	OutputDir  string
	Filename   string
	Subtitles  []string // paths to subtitle files
	Success    bool
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// DownloadSingle downloads one video (non-playlist).
func (d *Downloader) DownloadSingle(url string) DownloadResult {
	url = normalizeURL(url)
	return d.download(url, d.opts.OutputDir)
}

// DownloadPlaylist downloads a full playlist, creating a sub-directory.
func (d *Downloader) DownloadPlaylist(url string) []DownloadResult {
	url = normalizeURL(url)
	meta, err := d.fetchPlaylistMetadata(url)
	title := sanitizeDirName(meta.Title)
	if err != nil || title == "" {
		title = sanitizeDirName(url)
	}

	playlistDir := filepath.Join(d.opts.OutputDir, title)
	if err := os.MkdirAll(playlistDir, 0o755); err != nil {
		return []DownloadResult{{
			URL:     url,
			Success: false,
			Error:   fmt.Sprintf("cannot create playlist directory: %v", err),
		}}
	}

	if len(meta.Entries) == 0 {
		return []DownloadResult{d.download(url, playlistDir)}
	}

	stateEntries := make([]playliststate.EntryInput, 0, len(meta.Entries))
	for _, entry := range meta.Entries {
		stateEntries = append(stateEntries, playliststate.EntryInput{
			ID:    strings.TrimSpace(entry.ID),
			URL:   playlistEntryURL(entry),
			Title: strings.TrimSpace(entry.Title),
		})
	}
	stateMgr, err := playliststate.Open(
		playliststate.StatePath(playlistDir),
		url,
		title,
		playlistDir,
		stateEntries,
		d.opts.ResetPlaylistState,
	)
	if err != nil {
		return []DownloadResult{{
			URL:       url,
			OutputDir: playlistDir,
			Success:   false,
			Error:     fmt.Sprintf("cannot prepare playlist state: %v", err),
		}}
	}

	results := make([]DownloadResult, 0, len(meta.Entries))
	for _, entry := range stateMgr.Entries() {
		key := playlistStateKey(entry.ID, entry.URL)
		if entry.Status == playliststate.StatusSucceeded {
			if d.progress != nil {
				d.progress <- ProgressUpdate{
					Key:     key,
					VideoID: entry.ID,
					Title:   entry.Title,
					Status:  "skipped",
				}
			}
			continue
		}

		entryURL := strings.TrimSpace(entry.URL)
		if entryURL == "" {
			result := DownloadResult{
				VideoID:   entry.ID,
				Title:     entry.Title,
				URL:       url,
				OutputDir: playlistDir,
				Success:   false,
				Error:     "playlist entry is missing a downloadable URL",
			}
			_ = stateMgr.MarkFinished(key, entry.Title, "", nil, false, result.Error)
			results = append(results, result)
			continue
		}

		if err := stateMgr.MarkRunning(key); err != nil {
			results = append(results, DownloadResult{
				VideoID:   entry.ID,
				Title:     entry.Title,
				URL:       entryURL,
				OutputDir: playlistDir,
				Success:   false,
				Error:     fmt.Sprintf("cannot update playlist state: %v", err),
			})
			continue
		}

		result := d.download(entryURL, playlistDir)
		if strings.TrimSpace(result.Title) == "" {
			result.Title = entry.Title
		}
		if strings.TrimSpace(result.VideoID) == "" {
			result.VideoID = entry.ID
		}
		if err := stateMgr.MarkFinished(key, result.Title, result.Filename, result.Subtitles, result.Success, result.Error); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("download finished but failed to save playlist state: %v", err)
		}
		results = append(results, result)
	}
	return results
}

// download is the internal implementation that runs yt-dlp.
func (d *Downloader) download(url, outDir string) DownloadResult {
	bin, err := d.resolveYTDLPBin()
	if err != nil {
		return DownloadResult{URL: url, Success: false, Error: err.Error()}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return DownloadResult{URL: url, Success: false, Error: fmt.Sprintf("cannot create output dir: %v", err)}
	}

	args := d.buildArgs(url, outDir)
	cmd := exec.Command(bin, args...)
	cmd.Dir = outDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{URL: url, Success: false, Error: err.Error()}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{URL: url, Success: false, Error: err.Error()}
	}

	result := DownloadResult{
		URL:       url,
		OutputDir: outDir,
		StartedAt: time.Now(),
	}

	requestKey := url
	if d.progress != nil {
		d.progress <- ProgressUpdate{
			Key:     requestKey,
			Title:   url,
			Status:  "starting",
			Percent: 0,
		}
	}

	if err := cmd.Start(); err != nil {
		result.Error = err.Error()
		return result
	}

	// Collect stderr for error messages
	var stderrLines []string
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			stderrLines = append(stderrLines, sc.Text())
		}
	}()

	// Read stdout: yt-dlp emits progress lines AND JSON metadata lines
	var lastJSON VideoInfo
	sc := bufio.NewScanner(stdout)
	// Increase buffer size for large JSON lines
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()

		// Try to parse as JSON (video metadata from --print-json)
		if strings.HasPrefix(line, "{") {
			var info VideoInfo
			if jsonErr := json.Unmarshal([]byte(line), &info); jsonErr == nil {
				lastJSON = info
				if d.progress != nil {
					d.progress <- ProgressUpdate{
						Key:     progressKey(requestKey, info.ID, info.Title),
						VideoID: info.ID,
						Title:   info.Title,
						Status:  "merging",
						Percent: 100,
					}
				}
			}
			continue
		}

		// Parse progress line
		if m := progressRe.FindStringSubmatch(line); m != nil {
			var pct float64
			fmt.Sscanf(m[1], "%f", &pct)
			if d.progress != nil {
				d.progress <- ProgressUpdate{
					Key:     progressKey(requestKey, lastJSON.ID, lastJSON.Title),
					VideoID: lastJSON.ID,
					Title:   lastJSON.Title,
					Percent: pct,
					Speed:   m[2],
					ETA:     m[3],
					Status:  "downloading",
				}
			}
		}
	}

	cmdErr := cmd.Wait()
	result.FinishedAt = time.Now()

	if cmdErr != nil {
		result.Success = false
		errMsg := cmdErr.Error()
		if len(stderrLines) > 0 {
			errMsg = strings.Join(stderrLines, "; ")
		}
		result.Error = errMsg
		if d.progress != nil {
			d.progress <- ProgressUpdate{
				Key:     progressKey(requestKey, lastJSON.ID, lastJSON.Title),
				VideoID: lastJSON.ID,
				Title:   lastJSON.Title,
				Status:  "error",
				Error:   errMsg,
			}
		}
		return result
	}

	result.Success = true
	result.VideoID = lastJSON.ID
	result.Title = lastJSON.Title
	if lastJSON.Title != "" {
		// Reconstruct expected filename
		ext := strings.TrimSpace(d.opts.Format)
		if ext == "" {
			ext = strings.TrimSpace(lastJSON.Ext)
		}
		if ext != "" {
			result.Filename = filepath.Join(outDir,
				sanitizeFilename(lastJSON.Title)+"."+ext)
		}
	}

	// Collect subtitle files
	result.Subtitles = collectSubtitleFiles(outDir, lastJSON.Title)

	if d.progress != nil {
		d.progress <- ProgressUpdate{
			Key:     progressKey(requestKey, lastJSON.ID, lastJSON.Title),
			VideoID: lastJSON.ID,
			Title:   lastJSON.Title,
			Status:  "done",
			Percent: 100,
		}
	}

	return result
}

// fetchPlaylistMetadata uses yt-dlp --dump-single-json to get playlist metadata.
func (d *Downloader) fetchPlaylistMetadata(url string) (playlistMetadata, error) {
	bin, err := d.resolveYTDLPBin()
	if err != nil {
		return playlistMetadata{}, err
	}
	out, err := exec.Command(bin, "--dump-single-json", "--flat-playlist", url).Output()
	if err != nil {
		return playlistMetadata{}, err
	}
	var meta playlistMetadata
	if err := json.Unmarshal(out, &meta); err != nil {
		return playlistMetadata{}, err
	}
	return meta, nil
}

// collectSubtitleFiles globs for subtitle files that match a video title in dir.
func collectSubtitleFiles(dir, title string) []string {
	if title == "" {
		return nil
	}
	pattern := filepath.Join(dir, sanitizeFilename(title)+"*.vtt")
	matches, _ := filepath.Glob(pattern)
	pattern2 := filepath.Join(dir, sanitizeFilename(title)+"*.srt")
	m2, _ := filepath.Glob(pattern2)
	return append(matches, m2...)
}

// sanitizeDirName makes a string safe for use as a directory name.
func sanitizeDirName(s string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	s = re.ReplaceAllString(s, "_")
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// sanitizeFilename removes characters unsafe for filenames.
func sanitizeFilename(s string) string {
	return sanitizeDirName(s)
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	replacer := strings.NewReplacer(
		`\?`, `?`,
		`\=`, `=`,
		`\&`, `&`,
		`\#`, `#`,
	)
	return replacer.Replace(s)
}

func progressKey(requestKey, videoID, title string) string {
	switch {
	case videoID != "":
		return videoID
	case title != "":
		return title
	default:
		return requestKey
	}
}

func playlistStateKey(id, url string) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	return url
}

func playlistEntryURL(entry playlistEntry) string {
	switch {
	case entry.WebpageURL != "":
		return entry.WebpageURL
	case strings.HasPrefix(entry.URL, "http://"), strings.HasPrefix(entry.URL, "https://"):
		return entry.URL
	case isYouTubePlaylistEntry(entry) && entry.ID != "":
		// Flat YouTube playlist entries often only expose an id; reconstruct the watch URL.
		return "https://www.youtube.com/watch?v=" + entry.ID
	case isYouTubePlaylistEntry(entry) && entry.URL != "":
		return "https://www.youtube.com/watch?v=" + entry.URL
	default:
		// Non-YouTube extractors should provide webpage_url or an absolute url.
		// Avoid inventing a YouTube URL for other platforms.
		return ""
	}
}

func isYouTubePlaylistEntry(entry playlistEntry) bool {
	key := strings.ToLower(strings.TrimSpace(entry.IeKey))
	switch key {
	case "", "youtube", "youtube:tab", "youtube:playlist", "youtube:tabplaylist":
		return true
	default:
		return strings.HasPrefix(key, "youtube")
	}
}

func (d *Downloader) resolveYTDLPBin() (string, error) {
	return ytdlpbin.Resolve(d.opts.YTDLPBin)
}
