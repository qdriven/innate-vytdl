package record

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/innate/yt-dl/internal/downloader"
)

const catalogVersion = 1

// DownloadRecord is one download attempt. Subtitle paths live on the entry
// so the log is a single file.
type DownloadRecord struct {
	VideoID    string    `json:"video_id"    csv:"video_id"`
	Title      string    `json:"title"       csv:"title"`
	URL        string    `json:"url"         csv:"url"`
	OutputDir  string    `json:"output_dir"  csv:"output_dir"`
	Filename   string    `json:"filename"    csv:"filename"`
	Subtitles  []string  `json:"subtitles"   csv:"subtitles"`
	Success    bool      `json:"success"     csv:"success"`
	Error      string    `json:"error"       csv:"error"`
	StartedAt  time.Time `json:"started_at"  csv:"started_at"`
	FinishedAt time.Time `json:"finished_at" csv:"finished_at"`
	Duration   string    `json:"duration"    csv:"duration"`
}

// Catalog is the on-disk JSON document.
type Catalog struct {
	Version int              `json:"version"`
	Records []DownloadRecord `json:"records"`
}

// subtitleMapping is the legacy sidecar format (subtitle_mapping.json/.csv).
type subtitleMapping struct {
	VideoID   string   `json:"video_id"`
	Title     string   `json:"title"`
	VideoFile string   `json:"video_file"`
	Subtitles []string `json:"subtitles"`
}

// FromResult converts a downloader result into a catalog entry.
func FromResult(r downloader.DownloadResult) DownloadRecord {
	dur := r.FinishedAt.Sub(r.StartedAt).Round(time.Second).String()
	subs := r.Subtitles
	if subs == nil {
		subs = []string{}
	}
	return DownloadRecord{
		VideoID:    r.VideoID,
		Title:      r.Title,
		URL:        r.URL,
		OutputDir:  r.OutputDir,
		Filename:   r.Filename,
		Subtitles:  subs,
		Success:    r.Success,
		Error:      r.Error,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Duration:   dur,
	}
}

// Manager appends download attempts to one catalog file.
type Manager struct {
	format  string
	path    string
	mu      sync.Mutex
	pending []DownloadRecord
}

// HomeDir is the shared data root: $VYTDL_HOME or ~/.vytdl.
func HomeDir() string {
	if v := strings.TrimSpace(os.Getenv("VYTDL_HOME")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vytdl"
	}
	return filepath.Join(home, ".vytdl")
}

// CatalogName is the file name inside a data directory.
func CatalogName(format string) string {
	if normalizeFormat(format) == "csv" {
		return "download_record.csv"
	}
	return "download_record.json"
}

// DefaultPath is the catalog used by both the CLI and serve.
func DefaultPath(format string) string {
	return filepath.Join(HomeDir(), CatalogName(format))
}

// ResolvePath picks the catalog path. An empty recordFile uses DefaultPath.
// A bare name is placed in HomeDir; a path is used as given.
func ResolvePath(format, recordFile string) string {
	format = normalizeFormat(format)
	recordFile = strings.TrimSpace(recordFile)
	if recordFile == "" {
		return DefaultPath(format)
	}
	if filepath.IsAbs(recordFile) || strings.ContainsAny(recordFile, `/\`) {
		return ensureExt(recordFile, format)
	}
	return filepath.Join(HomeDir(), ensureExt(recordFile, format))
}

// NewManager opens a catalog writer. The file is created on Flush.
func NewManager(format, path string) (*Manager, error) {
	format = normalizeFormat(format)
	if format != "json" && format != "csv" {
		return nil, fmt.Errorf("unsupported log format %q: use json or csv", format)
	}
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("record path is required")
	}
	return &Manager{format: format, path: path}, nil
}

// Path returns the catalog file path.
func (m *Manager) Path() string { return m.path }

// Add queues one result. Call Flush to persist.
func (m *Manager) Add(r downloader.DownloadResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending = append(m.pending, FromResult(r))
}

// Flush appends queued results to the catalog.
func (m *Manager) Flush() error {
	m.mu.Lock()
	pending := append([]DownloadRecord(nil), m.pending...)
	m.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}

	unlock, err := lockCatalog(m.path)
	if err != nil {
		return err
	}
	defer unlock()

	existing, err := readCatalog(m.path, m.format)
	if err != nil {
		return err
	}
	merged := mergeNew(existing, pending)
	if err := writeCatalog(m.path, m.format, merged); err != nil {
		return err
	}

	m.mu.Lock()
	m.pending = dropFlushed(m.pending, pending)
	m.mu.Unlock()
	return nil
}

// List returns the persisted catalog plus any not-yet-flushed results.
func (m *Manager) List() ([]DownloadRecord, error) {
	unlock, err := lockCatalog(m.path)
	if err != nil {
		return nil, err
	}
	defer unlock()

	existing, err := readCatalog(m.path, m.format)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	pending := append([]DownloadRecord(nil), m.pending...)
	m.mu.Unlock()
	return append(existing, pending...), nil
}

// ImportLegacyDir folds download_record.* and subtitle_mapping.* from dir
// into this catalog, then removes those files. The live catalog file is
// never deleted. Already-imported attempts are skipped.
func (m *Manager) ImportLegacyDir(dir string) (int, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return 0, nil
	}

	unlock, err := lockCatalog(m.path)
	if err != nil {
		return 0, err
	}
	defer unlock()

	existing, err := readCatalog(m.path, m.format)
	if err != nil {
		return 0, err
	}

	type pair struct {
		record  string
		mapping string
		format  string
	}
	pairs := []pair{
		{filepath.Join(dir, "download_record.json"), filepath.Join(dir, "subtitle_mapping.json"), "json"},
		{filepath.Join(dir, "download_record.csv"), filepath.Join(dir, "subtitle_mapping.csv"), "csv"},
	}

	added := 0
	var remove []string
	for _, p := range pairs {
		recordSame := sameFile(p.record, m.path)
		recordExists := fileExists(p.record)
		mappingExists := fileExists(p.mapping)
		if !recordExists && !mappingExists {
			continue
		}
		if recordSame && !mappingExists {
			continue
		}

		var recs []DownloadRecord
		if recordExists && !recordSame {
			recs, err = readCatalog(p.record, p.format)
			if err != nil {
				return 0, err
			}
		}
		var maps []subtitleMapping
		if mappingExists {
			maps, err = readMappings(p.mapping, p.format)
			if err != nil {
				return 0, err
			}
		}

		if recordSame {
			n := fillSubtitles(existing, maps)
			added += n
		} else {
			recs = attachMappings(recs, maps)
			before := len(existing)
			existing = mergeNew(existing, recs)
			added += len(existing) - before
			if recordExists {
				remove = append(remove, p.record)
			}
		}
		if mappingExists {
			remove = append(remove, p.mapping)
		}
	}

	if added > 0 {
		if err := writeCatalog(m.path, m.format, existing); err != nil {
			return 0, err
		}
	}
	for _, path := range remove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return added, err
		}
	}
	return added, nil
}

func normalizeFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func ensureExt(path, format string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".csv":
		return path
	}
	if format == "csv" {
		return path + ".csv"
	}
	return path + ".json"
}

func mergeNew(existing, incoming []DownloadRecord) []DownloadRecord {
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[dedupeKey(r)] = struct{}{}
	}
	for _, r := range incoming {
		k := dedupeKey(r)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		existing = append(existing, r)
	}
	return existing
}

func dropFlushed(pending, flushed []DownloadRecord) []DownloadRecord {
	if len(pending) == 0 {
		return nil
	}
	gone := make(map[string]int, len(flushed))
	for _, r := range flushed {
		gone[dedupeKey(r)]++
	}
	out := pending[:0]
	for _, r := range pending {
		k := dedupeKey(r)
		if gone[k] > 0 {
			gone[k]--
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dedupeKey(r DownloadRecord) string {
	subs := strings.Join(r.Subtitles, "|")
	return strings.Join([]string{
		r.URL,
		r.StartedAt.UTC().Format(time.RFC3339Nano),
		r.Filename,
		fmt.Sprintf("%t", r.Success),
		r.Error,
		subs,
	}, "\x1f")
}

func attachMappings(recs []DownloadRecord, maps []subtitleMapping) []DownloadRecord {
	if len(maps) == 0 {
		return recs
	}
	for i := range recs {
		if len(recs[i].Subtitles) > 0 {
			continue
		}
		if i < len(maps) && sameIdentity(recs[i], maps[i]) {
			recs[i].Subtitles = cloneSubs(maps[i].Subtitles)
			continue
		}
		recs[i].Subtitles = cloneSubs(lookupMapping(recs[i], maps))
	}
	return recs
}

func fillSubtitles(recs []DownloadRecord, maps []subtitleMapping) int {
	if len(maps) == 0 {
		return 0
	}
	n := 0
	for i := range recs {
		if len(recs[i].Subtitles) > 0 {
			continue
		}
		var subs []string
		if i < len(maps) && sameIdentity(recs[i], maps[i]) {
			subs = maps[i].Subtitles
		} else {
			subs = lookupMapping(recs[i], maps)
		}
		if len(subs) == 0 {
			continue
		}
		recs[i].Subtitles = cloneSubs(subs)
		n++
	}
	return n
}

func lookupMapping(r DownloadRecord, maps []subtitleMapping) []string {
	for _, mp := range maps {
		if mp.VideoID != "" && mp.VideoID == r.VideoID && (mp.VideoFile == "" || r.Filename == "" || mp.VideoFile == r.Filename) {
			return mp.Subtitles
		}
	}
	for _, mp := range maps {
		if mp.VideoID == "" && mp.VideoFile != "" && mp.VideoFile == r.Filename {
			return mp.Subtitles
		}
	}
	return nil
}

func sameIdentity(r DownloadRecord, mp subtitleMapping) bool {
	if r.VideoID != "" || mp.VideoID != "" {
		if r.VideoID != mp.VideoID {
			return false
		}
		return mp.VideoFile == "" || r.Filename == "" || mp.VideoFile == r.Filename
	}
	return mp.VideoFile == "" || r.Filename == "" || mp.VideoFile == r.Filename
}

func cloneSubs(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func readCatalog(path, format string) ([]DownloadRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	if normalizeFormat(format) == "csv" || strings.EqualFold(filepath.Ext(path), ".csv") {
		return parseCSVRecords(data)
	}
	return parseJSONRecords(data)
}

func parseJSONRecords(data []byte) ([]DownloadRecord, error) {
	trim := strings.TrimSpace(string(data))
	if strings.HasPrefix(trim, "[") {
		var recs []DownloadRecord
		if err := json.Unmarshal(data, &recs); err != nil {
			return nil, err
		}
		return normalizeSubs(recs), nil
	}
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	return normalizeSubs(cat.Records), nil
}

func normalizeSubs(recs []DownloadRecord) []DownloadRecord {
	for i := range recs {
		if recs[i].Subtitles == nil {
			recs[i].Subtitles = []string{}
		}
	}
	return recs
}

func writeCatalog(path, format string, recs []DownloadRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var data []byte
	var err error
	if normalizeFormat(format) == "csv" {
		data, err = encodeCSVRecords(recs)
	} else {
		cat := Catalog{Version: catalogVersion, Records: normalizeSubs(recs)}
		if cat.Records == nil {
			cat.Records = []DownloadRecord{}
		}
		data, err = json.MarshalIndent(cat, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	}
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

var recordCSVHeader = []string{
	"video_id", "title", "url", "output_dir", "filename", "subtitles",
	"success", "error", "started_at", "finished_at", "duration",
}

func encodeCSVRecords(recs []DownloadRecord) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write(recordCSVHeader); err != nil {
		return nil, err
	}
	for _, r := range recs {
		row := []string{
			r.VideoID, r.Title, r.URL, r.OutputDir, r.Filename,
			strings.Join(r.Subtitles, "|"),
			fmt.Sprintf("%t", r.Success),
			r.Error,
			r.StartedAt.Format(time.RFC3339),
			r.FinishedAt.Format(time.RFC3339),
			r.Duration,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return []byte(b.String()), w.Error()
}

func parseCSVRecords(data []byte) ([]DownloadRecord, error) {
	rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil || len(rows) <= 1 {
		return nil, err
	}
	col := indexHeader(rows[0])
	recs := make([]DownloadRecord, 0, len(rows)-1)
	for _, row := range rows[1:] {
		startedAt, _ := time.Parse(time.RFC3339, colValue(row, col, "started_at"))
		finishedAt, _ := time.Parse(time.RFC3339, colValue(row, col, "finished_at"))
		var subs []string
		if raw := colValue(row, col, "subtitles"); raw != "" {
			subs = strings.Split(raw, "|")
		}
		if subs == nil {
			subs = []string{}
		}
		recs = append(recs, DownloadRecord{
			VideoID:    colValue(row, col, "video_id"),
			Title:      colValue(row, col, "title"),
			URL:        colValue(row, col, "url"),
			OutputDir:  colValue(row, col, "output_dir"),
			Filename:   colValue(row, col, "filename"),
			Subtitles:  subs,
			Success:    colValue(row, col, "success") == "true",
			Error:      colValue(row, col, "error"),
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			Duration:   colValue(row, col, "duration"),
		})
	}
	return recs, nil
}

func indexHeader(header []string) map[string]int {
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	// Legacy record CSV had no subtitles column. Positional fallback matches
	// the old header so those files still import.
	if _, ok := col["video_id"]; !ok && len(header) >= 10 {
		legacy := []string{
			"video_id", "title", "url", "output_dir", "filename",
			"success", "error", "started_at", "finished_at", "duration",
		}
		for i, name := range legacy {
			if i < len(header) {
				col[name] = i
			}
		}
	}
	return col
}

func colValue(row []string, col map[string]int, name string) string {
	i, ok := col[name]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func readMappings(path, format string) ([]subtitleMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if normalizeFormat(format) == "csv" || strings.EqualFold(filepath.Ext(path), ".csv") {
		return parseCSVMappings(data)
	}
	var maps []subtitleMapping
	if err := json.Unmarshal(data, &maps); err != nil {
		return nil, err
	}
	return maps, nil
}

func parseCSVMappings(data []byte) ([]subtitleMapping, error) {
	rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil || len(rows) <= 1 {
		return nil, err
	}
	col := indexHeader(rows[0])
	if _, ok := col["video_file"]; !ok && len(rows[0]) >= 4 {
		for i, name := range []string{"video_id", "title", "video_file", "subtitles"} {
			col[name] = i
		}
	}
	maps := make([]subtitleMapping, 0, len(rows)-1)
	for _, row := range rows[1:] {
		var subs []string
		if raw := colValue(row, col, "subtitles"); raw != "" {
			subs = strings.Split(raw, "|")
		}
		maps = append(maps, subtitleMapping{
			VideoID:   colValue(row, col, "video_id"),
			Title:     colValue(row, col, "title"),
			VideoFile: colValue(row, col, "video_file"),
			Subtitles: subs,
		})
	}
	return maps, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sameFile(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return aa == bb
}

func lockCatalog(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for record lock %s", lockPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
