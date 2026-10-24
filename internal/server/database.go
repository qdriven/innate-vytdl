// Package server implements the vYtDL background service (daemon): an HTTP
// API + WebSocket server that owns the download queue, SQLite persistence,
// VTT analysis and static frontend hosting. It is the single backend for the
// web UI (browser and Electron shell); see docs/refactor/sprint-0/.
package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DownloadStatus is the lifecycle state of a download record.
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
)

// DownloadRecord mirrors one row of the downloads table. Nullable columns are
// pointers so they serialize as null like the web/desktop frontends expect.
type DownloadRecord struct {
	ID              string  `json:"id"`
	URL             string  `json:"url"`
	Title           *string `json:"title"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	Speed           *string `json:"speed"`
	ETA             *string `json:"eta"`
	OutputDir       *string `json:"output_dir"`
	Filename        *string `json:"filename"`
	Subtitles       string  `json:"subtitles"`
	Error           *string `json:"error"`
	QueuePosition   int     `json:"queue_position"`
	CollectionID    *string `json:"collection_id"`
	CollectionTitle *string `json:"collection_title"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	Options         *string `json:"-"` // serialized StartDownloadRequest for resume
}

// VttReport mirrors one row of the vtt_reports table.
type VttReport struct {
	ID          string   `json:"id"`
	YoutubeURL  string   `json:"youtube_url"`
	VideoID     *string  `json:"video_id"`
	Title       *string  `json:"title"`
	Language    *string  `json:"language"`
	Content     string   `json:"content"`
	CueCount    int      `json:"cue_count"`
	DurationSec *float64 `json:"duration_sec"`
	CreatedAt   string   `json:"created_at"`
	Status      string   `json:"status"`
	Error       *string  `json:"error"`
}

// Database is the SQLite persistence layer (schema-compatible with the
// desktop/web apps; the options column powers resume-on-startup).
type Database struct {
	db *sql.DB
}

// OpenDatabase opens (creating if needed) the SQLite database at path.
func OpenDatabase(path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Single writer: modernc.org/sqlite is happiest with one connection for
	// this low-write workload, and it removes SQLITE_BUSY classes of bugs.
	db.SetMaxOpenConns(1)
	d := &Database{db: db}
	if err := d.init(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the database handle.
func (d *Database) Close() error { return d.db.Close() }

func (d *Database) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		title TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		progress REAL DEFAULT 0.0,
		speed TEXT,
		eta TEXT,
		output_dir TEXT,
		filename TEXT,
		subtitles TEXT DEFAULT '[]',
		error TEXT,
		queue_position INTEGER DEFAULT 0,
		collection_id TEXT,
		collection_title TEXT,
		options TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS vtt_reports (
		id TEXT PRIMARY KEY,
		youtube_url TEXT NOT NULL,
		video_id TEXT,
		title TEXT,
		language TEXT,
		content TEXT NOT NULL DEFAULT '',
		cue_count INTEGER DEFAULT 0,
		duration_sec REAL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'pending',
		error TEXT
	);`
	if _, err := d.db.Exec(schema); err != nil {
		return err
	}
	// Migration: add columns that older databases (web schema) may miss.
	for _, col := range []string{
		"ALTER TABLE downloads ADD COLUMN collection_id TEXT",
		"ALTER TABLE downloads ADD COLUMN collection_title TEXT",
		"ALTER TABLE downloads ADD COLUMN options TEXT",
	} {
		if _, err := d.db.Exec(col); err != nil {
			// "duplicate column name" means the column already exists.
			continue
		}
	}
	return nil
}

// nowISO renders timestamps like JavaScript's Date.prototype.toISOString.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z0700")
}

func nullStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func strPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// ── downloads ────────────────────────────────────────────────────────────────

const downloadCols = `id, url, title, status, progress, speed, eta, output_dir,
	filename, subtitles, error, queue_position, collection_id, collection_title,
	options, created_at, updated_at`

func scanDownload(row interface{ Scan(...any) error }) (DownloadRecord, error) {
	var r DownloadRecord
	var progress sql.NullFloat64
	var queuePos sql.NullInt64
	err := row.Scan(&r.ID, &r.URL, &r.Title, &r.Status, &progress, &r.Speed, &r.ETA,
		&r.OutputDir, &r.Filename, &r.Subtitles, &r.Error, &queuePos,
		&r.CollectionID, &r.CollectionTitle, &r.Options, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	r.Progress = progress.Float64
	r.QueuePosition = int(queuePos.Int64)
	return r, nil
}

const downloadSelect = `SELECT ` + downloadCols + ` FROM downloads`

// CreateDownload inserts a new download record.
func (d *Database) CreateDownload(r DownloadRecord) error {
	_, err := d.db.Exec(
		`INSERT INTO downloads (`+downloadCols+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.URL, strPtr(r.Title), r.Status, r.Progress, strPtr(r.Speed), strPtr(r.ETA),
		strPtr(r.OutputDir), strPtr(r.Filename), r.Subtitles, strPtr(r.Error), r.QueuePosition,
		strPtr(r.CollectionID), strPtr(r.CollectionTitle), strPtr(r.Options),
		r.CreatedAt, r.UpdatedAt)
	return err
}

// GetAllDownloads lists every download, newest first.
func (d *Database) GetAllDownloads() ([]DownloadRecord, error) {
	rows, err := d.db.Query(downloadSelect + " ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DownloadRecord
	for rows.Next() {
		r, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDownloadByID fetches one download, nil when missing.
func (d *Database) GetDownloadByID(id string) (*DownloadRecord, error) {
	r, err := scanDownload(d.db.QueryRow(downloadSelect+" WHERE id = ?", id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) execUpdate(query string, args ...any) error {
	_, err := d.db.Exec(query, args...)
	return err
}

// UpdateDownloadStatus changes only the status column.
func (d *Database) UpdateDownloadStatus(id string, status DownloadStatus) error {
	return d.execUpdate(
		"UPDATE downloads SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		string(status), id)
}

// UpdateDownloadProgress stores the latest progress sample.
func (d *Database) UpdateDownloadProgress(id string, progress float64, speed, eta *string) error {
	return d.execUpdate(
		"UPDATE downloads SET progress = ?, speed = ?, eta = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		progress, strPtr(speed), strPtr(eta), id)
}

// UpdateDownloadComplete marks a download finished with its output metadata.
func (d *Database) UpdateDownloadComplete(id, title, filename, subtitles string) error {
	return d.execUpdate(
		`UPDATE downloads SET title = ?, filename = ?, subtitles = ?,
		 status = 'completed', progress = 100.0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		title, filename, subtitles, id)
}

// UpdateDownloadError marks a download failed.
func (d *Database) UpdateDownloadError(id string, errMsg string) error {
	return d.execUpdate(
		"UPDATE downloads SET status = 'failed', error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		errMsg, id)
}

// UpdateQueuePosition stores the pending queue position.
func (d *Database) UpdateQueuePosition(id string, position int) error {
	return d.execUpdate(
		"UPDATE downloads SET queue_position = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		position, id)
}

// DeleteDownload removes one download record.
func (d *Database) DeleteDownload(id string) error {
	return d.execUpdate("DELETE FROM downloads WHERE id = ?", id)
}

// GetResumableDownloads returns downloads that were in flight when the daemon
// last stopped: everything still pending plus anything marked downloading.
func (d *Database) GetResumableDownloads() ([]DownloadRecord, error) {
	rows, err := d.db.Query(downloadSelect + " WHERE status IN ('pending', 'downloading') ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DownloadRecord
	for rows.Next() {
		r, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── settings ─────────────────────────────────────────────────────────────────

// GetSetting returns a setting value, "" when unset.
func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting upserts a setting.
func (d *Database) SetSetting(key, value string) error {
	_, err := d.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value)
	return err
}

// ── vtt reports ──────────────────────────────────────────────────────────────

const vttCols = `id, youtube_url, video_id, title, language, content, cue_count,
	duration_sec, created_at, status, error`

func scanVttReport(row interface{ Scan(...any) error }) (VttReport, error) {
	var r VttReport
	var videoID, title, language sql.NullString
	var duration sql.NullFloat64
	err := row.Scan(&r.ID, &r.YoutubeURL, &videoID, &title, &language, &r.Content,
		&r.CueCount, &duration, &r.CreatedAt, &r.Status, &r.Error)
	if err != nil {
		return r, err
	}
	r.VideoID = nullStr(videoID)
	r.Title = nullStr(title)
	r.Language = nullStr(language)
	r.DurationSec = nil
	if duration.Valid {
		r.DurationSec = &duration.Float64
	}
	return r, nil
}

const vttSelect = `SELECT ` + vttCols + ` FROM vtt_reports`

// CreateVttReport inserts a new analysis report row.
func (d *Database) CreateVttReport(r VttReport) error {
	_, err := d.db.Exec(
		`INSERT INTO vtt_reports (`+vttCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.YoutubeURL, strPtr(r.VideoID), strPtr(r.Title), strPtr(r.Language),
		r.Content, r.CueCount, r.DurationSec, r.CreatedAt, r.Status, strPtr(r.Error))
	return err
}

// GetVttReport fetches one report, nil when missing.
func (d *Database) GetVttReport(id string) (*VttReport, error) {
	r, err := scanVttReport(d.db.QueryRow(vttSelect+" WHERE id = ?", id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListVttReports returns one page of reports plus the total count.
func (d *Database) ListVttReports(page, limit int, lang string) ([]VttReport, int, error) {
	where, args := "", []any{}
	if lang != "" {
		where = " WHERE language = ?"
		args = append(args, lang)
	}
	var total int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM vtt_reports"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := vttSelect + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, (page-1)*limit)
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []VttReport
	for rows.Next() {
		r, err := scanVttReport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// VttReportUpdate carries partial updates for one report.
type VttReportUpdate struct {
	Title       *string
	Language    *string
	Content     *string
	CueCount    *int
	DurationSec *float64
	Status      *string
	Error       *string
	VideoID     *string
}

// UpdateVttReport applies partial updates to one report.
func (d *Database) UpdateVttReport(id string, u VttReportUpdate) error {
	sets := []string{}
	args := []any{}
	if u.Title != nil {
		sets, args = append(sets, "title = ?"), append(args, *u.Title)
	}
	if u.Language != nil {
		sets, args = append(sets, "language = ?"), append(args, *u.Language)
	}
	if u.Content != nil {
		sets, args = append(sets, "content = ?"), append(args, *u.Content)
	}
	if u.CueCount != nil {
		sets, args = append(sets, "cue_count = ?"), append(args, *u.CueCount)
	}
	if u.DurationSec != nil {
		sets, args = append(sets, "duration_sec = ?"), append(args, *u.DurationSec)
	}
	if u.Status != nil {
		sets, args = append(sets, "status = ?"), append(args, *u.Status)
	}
	if u.Error != nil {
		sets, args = append(sets, "error = ?"), append(args, *u.Error)
	}
	if u.VideoID != nil {
		sets, args = append(sets, "video_id = ?"), append(args, *u.VideoID)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	return d.execUpdate("UPDATE vtt_reports SET "+joinComma(sets)+" WHERE id = ?", args...)
}

// DeleteVttReport removes one report.
func (d *Database) DeleteVttReport(id string) error {
	return d.execUpdate("DELETE FROM vtt_reports WHERE id = ?", id)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// marshalSubtitles serializes a subtitle path list the way the web backend did.
func marshalSubtitles(paths []string) string {
	if paths == nil {
		paths = []string{}
	}
	b, err := json.Marshal(paths)
	if err != nil {
		return "[]"
	}
	return string(b)
}
