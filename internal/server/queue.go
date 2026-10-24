package server

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/innate/yt-dl/internal/downloader"
)

// StartDownloadRequest mirrors the frontend DownloadOptions payload
// (apps/vytdl-desktop/src/types/index.ts). Unknown fields are ignored by the
// JSON decoder so older/newer frontends stay compatible.
type StartDownloadRequest struct {
	URL                 string        `json:"url"`
	Title               *string       `json:"title"`
	CollectionID        *string       `json:"collection_id"`
	CollectionTitle     *string       `json:"collection_title"`
	IsPlaylist          bool          `json:"is_playlist"`
	Quality             *string       `json:"quality"`
	FormatID            *string       `json:"format_id"`
	Format              *string       `json:"format"`
	OutputDir           *string       `json:"output_dir"`
	SubLangs            []string      `json:"sub_langs"`
	WriteSubs           *bool         `json:"write_subs"`
	WriteAutoSubs       *bool         `json:"write_auto_subs"`
	StartTime           *string       `json:"start_time"`
	EndTime             *string       `json:"end_time"`
	Cookie              *CookieConfig `json:"cookie"`
	Proxy               *string       `json:"proxy"`
	RateLimit           *string       `json:"rate_limit"`
	ConcurrentFragments *int          `json:"concurrent_fragments"`
	EmbedThumbnail      *bool         `json:"embed_thumbnail"`
	EmbedMetadata       *bool         `json:"embed_metadata"`
	EmbedChapters       *bool         `json:"embed_chapters"`
	SponsorblockRemove  *bool         `json:"sponsorblock_remove"`
	FilenameTemplate    *string       `json:"filename_template"`
	POToken             *string       `json:"po_token"`
	ExtractorArgs       *string       `json:"extractor_args"`
	ConfigLocation      *string       `json:"config_location"`
}

// CookieConfig is the frontend's { mode: "file" | "browser" } cookie source.
type CookieConfig struct {
	Mode    string  `json:"mode"` // "file" or "browser"
	Path    *string `json:"path"`
	Browser *string `json:"browser"`
}

// ToDownloaderOptions maps the request onto the shared downloader options.
func (r *StartDownloadRequest) ToDownloaderOptions(defaultOutputDir string) downloader.Options {
	opts := downloader.DefaultOptions()
	opts.URL = r.URL
	opts.IsPlaylist = r.IsPlaylist
	opts.OutputDir = defaultOutputDir
	if r.OutputDir != nil && strings.TrimSpace(*r.OutputDir) != "" {
		opts.OutputDir = *r.OutputDir
	}
	if r.Format != nil {
		opts.Format = *r.Format
	}
	// Quality: "best" and absent → downloader default; numeric presets and raw
	// selectors pass through; format_id (Format Picker) takes priority.
	opts.Quality = ""
	if r.Quality != nil && *r.Quality != "" && *r.Quality != "best" {
		opts.Quality = *r.Quality
	}
	if r.FormatID != nil && strings.TrimSpace(*r.FormatID) != "" {
		opts.FormatID = strings.TrimSpace(*r.FormatID)
	}
	if r.SubLangs != nil {
		opts.SubtitleLangs = r.SubLangs
	}
	// Subtitle flags default on (CLI/desktop convention) when the request omits them.
	opts.WriteSubtitles = r.WriteSubs == nil || *r.WriteSubs
	opts.WriteAutoSubs = r.WriteAutoSubs == nil || *r.WriteAutoSubs
	if r.WriteAutoSubs != nil && *r.WriteAutoSubs {
		opts.SleepSubtitles = "1" // 坑知识 #3
	}
	if r.StartTime != nil {
		opts.StartTime = *r.StartTime
	}
	if r.EndTime != nil {
		opts.EndTime = *r.EndTime
	}
	if opts.StartTime != "" || opts.EndTime != "" {
		opts.SectionSafeSelector = true // 坑知识 #1
	}
	if r.Cookie != nil {
		switch r.Cookie.Mode {
		case "file":
			if r.Cookie.Path != nil {
				opts.CookiesFile = *r.Cookie.Path
			}
		case "browser":
			if r.Cookie.Browser != nil {
				opts.CookiesFromBrowser = *r.Cookie.Browser
			}
		}
	}
	if r.Proxy != nil {
		opts.Proxy = *r.Proxy
	}
	if r.RateLimit != nil {
		opts.RateLimit = *r.RateLimit
	}
	if r.ConcurrentFragments != nil && *r.ConcurrentFragments > 0 {
		opts.ConcurrentFragments = *r.ConcurrentFragments
	}
	setBool := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	setBool(&opts.EmbedThumbnail, r.EmbedThumbnail)
	setBool(&opts.EmbedMetadata, r.EmbedMetadata)
	setBool(&opts.EmbedChapters, r.EmbedChapters)
	setBool(&opts.SponsorblockRemove, r.SponsorblockRemove)
	if r.FilenameTemplate != nil {
		opts.FilenameTemplate = *r.FilenameTemplate
	}
	if r.POToken != nil {
		opts.POToken = *r.POToken
	}
	if r.ExtractorArgs != nil {
		opts.ExtractorArgs = *r.ExtractorArgs
	}
	if r.ConfigLocation != nil && strings.TrimSpace(*r.ConfigLocation) != "" {
		opts.ConfigLocation = *r.ConfigLocation
	} else {
		opts.IgnoreUserConfig = true // 坑知识 #6
	}
	opts.WindowsFilenames = true
	return opts
}

type pendingItem struct {
	id   string
	opts downloader.Options
}

type activeDownload struct {
	cancel  context.CancelFunc
	watcher context.CancelFunc // stops the drain goroutines
}

// QueueManager runs the FIFO download queue with a concurrency cap, mirroring
// queue.rs / queue.ts behavior: pending → downloading → completed/failed/cancelled,
// queue_position persistence, cancel of pending and active items, and
// resume-on-startup.
type QueueManager struct {
	db  *Database
	hub *Hub
	// defaultOutputDir backs resumed records that have no stored output dir.
	defaultOutputDir string

	mu            sync.Mutex
	maxConcurrent int
	pending       []pendingItem
	active        map[string]*activeDownload

	wg sync.WaitGroup
}

// NewQueueManager creates the queue; call LoadSettings and ResumeOnStartup
// before serving traffic.
func NewQueueManager(db *Database, hub *Hub, defaultOutputDir string) *QueueManager {
	return &QueueManager{
		db:               db,
		hub:              hub,
		defaultOutputDir: defaultOutputDir,
		maxConcurrent:    3,
		active:           make(map[string]*activeDownload),
	}
}

// LoadSettings applies the persisted concurrency cap.
func (q *QueueManager) LoadSettings() {
	if v, err := q.db.GetSetting("max_concurrent_downloads"); err == nil && v != "" {
		q.setMaxConcurrentUnchecked(v)
	}
}

func (q *QueueManager) setMaxConcurrentUnchecked(v string) {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		n = 3
	}
	q.maxConcurrent = clampInt(n, 1, 10)
}

// MaxConcurrent returns the current concurrency cap.
func (q *QueueManager) MaxConcurrent() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxConcurrent
}

// SetMaxConcurrent updates the cap (clamped 1..10), persists it, and kicks the queue.
func (q *QueueManager) SetMaxConcurrent(n int) {
	q.mu.Lock()
	q.maxConcurrent = clampInt(n, 1, 10)
	capped := q.maxConcurrent
	q.mu.Unlock()
	_ = q.db.SetSetting("max_concurrent_downloads", strconv.Itoa(capped))
	q.processQueue()
}

// Enqueue appends a download and starts it if capacity allows.
func (q *QueueManager) Enqueue(id string, opts downloader.Options) {
	// Pin the configured yt-dlp binary (same one metadata calls use).
	if strings.TrimSpace(opts.YTDLPBin) == "" {
		if v, err := q.db.GetSetting("yt_dlp_path"); err == nil {
			opts.YTDLPBin = v
		}
	}
	q.mu.Lock()
	q.pending = append(q.pending, pendingItem{id: id, opts: opts})
	q.mu.Unlock()
	_ = q.db.UpdateDownloadStatus(id, StatusPending)
	q.updatePendingPositions()
	q.hub.Broadcast("queue:update", map[string]any{"id": id, "status": "pending"})
	q.processQueue()
}

// Cancel aborts a pending or active download (no-op for unknown ids, matching
// the web backend which always returns success).
func (q *QueueManager) Cancel(id string) {
	q.mu.Lock()
	for i, p := range q.pending {
		if p.id == id {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			q.mu.Unlock()
			_ = q.db.UpdateDownloadStatus(id, StatusCancelled)
			q.updatePendingPositions()
			q.hub.Broadcast("download:status:"+id, "cancelled")
			q.hub.Broadcast("queue:update", map[string]any{"id": id, "status": "cancelled"})
			return
		}
	}
	active := q.active[id]
	q.mu.Unlock()

	if active != nil {
		// Status is updated when the download task finishes.
		active.cancel()
		return
	}
	_ = q.db.UpdateDownloadStatus(id, StatusCancelled)
	q.hub.Broadcast("download:status:"+id, "cancelled")
}

// ResumeOnStartup re-enqueues downloads left in flight by a previous run:
// 'pending' rows are re-enqueued as-is, 'downloading' rows are reset to
// pending first (desktop resume behavior).
func (q *QueueManager) ResumeOnStartup() error {
	rows, err := q.db.GetResumableDownloads()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == string(StatusDownloading) {
			if err := q.db.UpdateDownloadStatus(row.ID, StatusPending); err != nil {
				return err
			}
		}
		if row.Options == nil || *row.Options == "" {
			_ = q.db.UpdateDownloadError(row.ID, "cannot resume: no saved download options")
			continue
		}
		var req StartDownloadRequest
		if err := json.Unmarshal([]byte(*row.Options), &req); err != nil {
			_ = q.db.UpdateDownloadError(row.ID, "cannot resume: invalid saved options")
			continue
		}
		outputDir := derefOr(row.OutputDir, q.defaultOutputDir)
		opts := req.ToDownloaderOptions(outputDir)
		q.Enqueue(row.ID, opts)
	}
	return nil
}

// Shutdown cancels all active downloads and waits for them to finish.
func (q *QueueManager) Shutdown() {
	q.mu.Lock()
	for _, a := range q.active {
		a.cancel()
	}
	q.mu.Unlock()
	q.wg.Wait()
}

func (q *QueueManager) updatePendingPositions() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, p := range q.pending {
		_ = q.db.UpdateQueuePosition(p.id, i+1)
	}
}

func (q *QueueManager) processQueue() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.active) < q.maxConcurrent && len(q.pending) > 0 {
		next := q.pending[0]
		q.pending = q.pending[1:]
		q.startLocked(next)
	}
}

func (q *QueueManager) startLocked(item pendingItem) {
	_ = q.db.UpdateDownloadStatus(item.id, StatusDownloading)
	_ = q.db.UpdateQueuePosition(item.id, 0)
	q.hub.Broadcast("download:status:"+item.id, "downloading")
	q.hub.Broadcast("queue:update", map[string]any{"id": item.id, "status": "downloading"})

	progressCh := make(chan downloader.ProgressUpdate, 128)
	logsCh := make(chan downloader.LogLine, 256)
	dl := downloader.New(item.opts, progressCh)
	dl.SetLogs(logsCh)
	q.active[item.id] = &activeDownload{cancel: dl.Cancel}

	// Drains end when the download goroutine closes the channels.
	var drainWg sync.WaitGroup
	drainWg.Add(2)
	go func() {
		defer drainWg.Done()
		q.drainProgress(item.id, progressCh)
	}()
	go func() {
		defer drainWg.Done()
		q.drainLogs(item.id, logsCh)
	}()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		var results []downloader.DownloadResult
		if item.opts.IsPlaylist {
			results = dl.DownloadPlaylist(item.opts.URL)
		} else {
			results = []downloader.DownloadResult{dl.DownloadSingle(item.opts.URL)}
		}
		// The downloader is synchronous inside this goroutine, so no sends are
		// in flight anymore; closing wakes the drains.
		close(progressCh)
		close(logsCh)
		drainWg.Wait()
		q.finish(item.id, results)

		q.mu.Lock()
		delete(q.active, item.id)
		q.mu.Unlock()
		q.updatePendingPositions()
		q.processQueue()
	}()
}

func (q *QueueManager) drainProgress(id string, ch <-chan downloader.ProgressUpdate) {
	for pu := range ch {
		switch pu.Status {
		case "downloading":
			speed, eta := pu.Speed, pu.ETA
			_ = q.db.UpdateDownloadProgress(id, pu.Percent, &speed, &eta)
		case "error":
			// Errors (including cancellation) are handled from the results.
		}
		q.hub.Broadcast("download:progress:"+id, map[string]any{
			"video_id": nullableString(pu.VideoID),
			"title":    nullableString(pu.Title),
			"percent":  pu.Percent,
			"speed":    nullableString(pu.Speed),
			"eta":      nullableString(pu.ETA),
			"status":   pu.Status,
			"error":    nullableString(pu.Error),
		})
	}
}

func (q *QueueManager) drainLogs(id string, ch <-chan downloader.LogLine) {
	for line := range ch {
		q.hub.Broadcast("download:log:"+id, map[string]any{
			"level":   line.Level,
			"message": line.Message,
		})
	}
}

// finish persists the final status for a finished download (aggregating
// per-entry results for playlists).
func (q *QueueManager) finish(id string, results []downloader.DownloadResult) {
	if len(results) == 0 {
		_ = q.db.UpdateDownloadError(id, "download produced no results")
		q.hub.Broadcast("download:error:"+id, map[string]any{"success": false, "error": "download produced no results"})
		q.hub.Broadcast("download:status:"+id, "failed")
		return
	}

	cancelled, failed := false, false
	firstError := ""
	var title, filename string
	var subtitles []string
	for _, r := range results {
		if r.Cancelled {
			cancelled = true
			break
		}
		if !r.Success {
			failed = true
			if firstError == "" {
				firstError = r.Error
			}
			continue
		}
		if title == "" && r.Title != "" {
			title = r.Title
		}
		if filename == "" && r.Filename != "" {
			filename = r.Filename
		}
		subtitles = append(subtitles, r.Subtitles...)
	}

	switch {
	case cancelled:
		_ = q.db.UpdateDownloadStatus(id, StatusCancelled)
		q.hub.Broadcast("download:status:"+id, "cancelled")
	case failed:
		msg := firstError
		if len(results) > 1 {
			ok := 0
			for _, r := range results {
				if r.Success {
					ok++
				}
			}
			msg = strconv.Itoa(len(results)-ok) + "/" + strconv.Itoa(len(results)) + " entries failed: " + firstError
		}
		_ = q.db.UpdateDownloadError(id, msg)
		q.hub.Broadcast("download:error:"+id, map[string]any{"success": false, "error": msg})
		q.hub.Broadcast("download:status:"+id, "failed")
	default:
		if title == "" {
			title = "Downloaded"
		}
		_ = q.db.UpdateDownloadComplete(id, title, filename, marshalSubtitles(subtitles))
		q.hub.Broadcast("download:complete:"+id, map[string]any{
			"success": true,
			"data": map[string]any{
				"title":     title,
				"filename":  filename,
				"subtitles": subtitles,
			},
		})
		q.hub.Broadcast("download:status:"+id, "completed")
	}
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func derefOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
