package record

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sync"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/innate/yt-dl/internal/downloader"
)

// DownloadRecord is a single entry in the download log.
type DownloadRecord struct {
	VideoID    string    `json:"video_id"    csv:"video_id"`
	Title      string    `json:"title"       csv:"title"`
	URL        string    `json:"url"         csv:"url"`
	OutputDir  string    `json:"output_dir"  csv:"output_dir"`
	Filename   string    `json:"filename"    csv:"filename"`
	Success    bool      `json:"success"     csv:"success"`
	Error      string    `json:"error"       csv:"error"`
	StartedAt  time.Time `json:"started_at"  csv:"started_at"`
	FinishedAt time.Time `json:"finished_at" csv:"finished_at"`
	Duration   string    `json:"duration"    csv:"duration"`
}

// SubtitleMapping associates a video with its subtitle files.
type SubtitleMapping struct {
	VideoID   string   `json:"video_id"  csv:"video_id"`
	Title     string   `json:"title"     csv:"title"`
	VideoFile string   `json:"video_file" csv:"video_file"`
	Subtitles []string `json:"subtitles" csv:"subtitles"`
}

// FromResult converts a downloader result to a DownloadRecord.
func FromResult(r downloader.DownloadResult) DownloadRecord {
	dur := r.FinishedAt.Sub(r.StartedAt).Round(time.Second).String()
	return DownloadRecord{
		VideoID:    r.VideoID,
		Title:      r.Title,
		URL:        r.URL,
		OutputDir:  r.OutputDir,
		Filename:   r.Filename,
		Success:    r.Success,
		Error:      r.Error,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Duration:   dur,
	}
}

// MappingFromResult converts a result to a SubtitleMapping.
func MappingFromResult(r downloader.DownloadResult) SubtitleMapping {
	return SubtitleMapping{
		VideoID:   r.VideoID,
		Title:     r.Title,
		VideoFile: r.Filename,
		Subtitles: r.Subtitles,
	}
}

// Manager maintains an in-memory list of records and mappings,
// and flushes them to disk in JSON or CSV format.
type Manager struct {
	format      string             // "json" or "csv"
	recordPath  string
	mappingPath string
	mu          sync.Mutex
	records     []DownloadRecord
	mappings    []SubtitleMapping
}

// NewManager creates a record manager. format is "json" or "csv".
// baseRecord / baseMapping are base filenames without extensions.
func NewManager(format, baseRecord, baseMapping, dir string) *Manager {
	ext := "." + format
	m := &Manager{
		format:      format,
		recordPath:  filepath.Join(dir, baseRecord+ext),
		mappingPath: filepath.Join(dir, baseMapping+ext),
	}
	m.loadExisting()
	return m
}

// Add appends a result to both the record list and mapping list.
func (m *Manager) Add(r downloader.DownloadResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, FromResult(r))
	m.mappings = append(m.mappings, MappingFromResult(r))
}

// Flush writes all records and mappings to disk.
func (m *Manager) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.writeRecords(); err != nil {
		return fmt.Errorf("write records: %w", err)
	}
	if err := m.writeMappings(); err != nil {
		return fmt.Errorf("write mappings: %w", err)
	}
	return nil
}

func (m *Manager) writeRecords() error {
	if m.format == "csv" {
		return writeCSVRecords(m.recordPath, m.records)
	}
	return writeJSON(m.recordPath, m.records)
}

func (m *Manager) writeMappings() error {
	if m.format == "csv" {
		return writeCSVMappings(m.mappingPath, m.mappings)
	}
	return writeJSON(m.mappingPath, m.mappings)
}

// RecordPath returns the resolved record file path.
func (m *Manager) RecordPath() string { return m.recordPath }

// MappingPath returns the resolved mapping file path.
func (m *Manager) MappingPath() string { return m.mappingPath }

func (m *Manager) loadExisting() {
	if m.format == "csv" {
		m.records = readCSVRecords(m.recordPath)
		m.mappings = readCSVMappings(m.mappingPath)
		return
	}
	m.records = readJSONRecords(m.recordPath)
	m.mappings = readJSONMappings(m.mappingPath)
}

// ---- JSON helpers ----

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---- CSV helpers ----

var recordCSVHeader = []string{
	"video_id", "title", "url", "output_dir", "filename",
	"success", "error", "started_at", "finished_at", "duration",
}

func writeCSVRecords(path string, records []DownloadRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write(recordCSVHeader)
	for _, r := range records {
		row := []string{
			r.VideoID, r.Title, r.URL, r.OutputDir, r.Filename,
			fmt.Sprintf("%t", r.Success),
			r.Error,
			r.StartedAt.Format(time.RFC3339),
			r.FinishedAt.Format(time.RFC3339),
			r.Duration,
		}
		_ = w.Write(row)
	}
	w.Flush()
	return w.Error()
}

var mappingCSVHeader = []string{"video_id", "title", "video_file", "subtitles"}

func writeCSVMappings(path string, mappings []SubtitleMapping) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write(mappingCSVHeader)
	for _, m := range mappings {
		row := []string{
			m.VideoID, m.Title, m.VideoFile,
			strings.Join(m.Subtitles, "|"),
		}
		_ = w.Write(row)
	}
	w.Flush()
	return w.Error()
}

func readJSONRecords(path string) []DownloadRecord {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var records []DownloadRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil
	}
	return records
}

func readJSONMappings(path string) []SubtitleMapping {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var mappings []SubtitleMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil
	}
	return mappings
}

func readCSVRecords(path string) []DownloadRecord {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) <= 1 {
		return nil
	}

	records := make([]DownloadRecord, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 10 {
			continue
		}
		startedAt, _ := time.Parse(time.RFC3339, row[7])
		finishedAt, _ := time.Parse(time.RFC3339, row[8])
		records = append(records, DownloadRecord{
			VideoID:    row[0],
			Title:      row[1],
			URL:        row[2],
			OutputDir:  row[3],
			Filename:   row[4],
			Success:    row[5] == "true",
			Error:      row[6],
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			Duration:   row[9],
		})
	}
	return records
}

func readCSVMappings(path string) []SubtitleMapping {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) <= 1 {
		return nil
	}

	mappings := make([]SubtitleMapping, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 4 {
			continue
		}
		var subtitles []string
		if row[3] != "" {
			subtitles = strings.Split(row[3], "|")
		}
		mappings = append(mappings, SubtitleMapping{
			VideoID:   row[0],
			Title:     row[1],
			VideoFile: row[2],
			Subtitles: subtitles,
		})
	}
	return mappings
}