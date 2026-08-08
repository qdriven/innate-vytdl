package record

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/innate/yt-dl/internal/downloader"
)

func TestManagerFlushJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := NewManager("json", "downloads", "mapping", dir)
	mgr.Add(sampleResult(dir))

	if err := mgr.Flush(); err != nil {
		t.Fatalf("flush json: %v", err)
	}

	recordFile, err := os.ReadFile(filepath.Join(dir, "downloads.json"))
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	var records []DownloadRecord
	if err := json.Unmarshal(recordFile, &records); err != nil {
		t.Fatalf("unmarshal records: %v", err)
	}
	if len(records) != 1 || !records[0].Success {
		t.Fatalf("unexpected records: %#v", records)
	}

	mappingFile, err := os.ReadFile(filepath.Join(dir, "mapping.json"))
	if err != nil {
		t.Fatalf("read mapping file: %v", err)
	}
	var mappings []SubtitleMapping
	if err := json.Unmarshal(mappingFile, &mappings); err != nil {
		t.Fatalf("unmarshal mappings: %v", err)
	}
	if len(mappings) != 1 || len(mappings[0].Subtitles) != 2 {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestManagerFlushCSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := NewManager("csv", "downloads", "mapping", dir)
	mgr.Add(sampleResult(dir))

	if err := mgr.Flush(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}

	recordFH, err := os.Open(filepath.Join(dir, "downloads.csv"))
	if err != nil {
		t.Fatalf("open record csv: %v", err)
	}
	defer recordFH.Close()

	recordRows, err := csv.NewReader(recordFH).ReadAll()
	if err != nil {
		t.Fatalf("read record csv: %v", err)
	}
	if len(recordRows) != 2 || recordRows[1][5] != "true" {
		t.Fatalf("unexpected record rows: %#v", recordRows)
	}

	mappingFH, err := os.Open(filepath.Join(dir, "mapping.csv"))
	if err != nil {
		t.Fatalf("open mapping csv: %v", err)
	}
	defer mappingFH.Close()

	mappingRows, err := csv.NewReader(mappingFH).ReadAll()
	if err != nil {
		t.Fatalf("read mapping csv: %v", err)
	}
	if len(mappingRows) != 2 || mappingRows[1][3] == "" {
		t.Fatalf("unexpected mapping rows: %#v", mappingRows)
	}
}

func TestManagerLoadsExistingJSONOnRerun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := NewManager("json", "downloads", "mapping", dir)
	first.Add(sampleResult(dir))
	if err := first.Flush(); err != nil {
		t.Fatalf("first flush: %v", err)
	}

	second := NewManager("json", "downloads", "mapping", dir)
	next := sampleResult(dir)
	next.VideoID = "def456"
	next.Title = "Second Video"
	second.Add(next)
	if err := second.Flush(); err != nil {
		t.Fatalf("second flush: %v", err)
	}

	recordFile, err := os.ReadFile(filepath.Join(dir, "downloads.json"))
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	var records []DownloadRecord
	if err := json.Unmarshal(recordFile, &records); err != nil {
		t.Fatalf("unmarshal records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 persisted records, got %#v", records)
	}
}

func sampleResult(dir string) downloader.DownloadResult {
	start := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Second)
	return downloader.DownloadResult{
		VideoID:    "abc123",
		Title:      "Sample Video",
		URL:        "https://example.com/watch?v=abc123",
		OutputDir:  dir,
		Filename:   filepath.Join(dir, "Sample Video.mp4"),
		Subtitles:  []string{filepath.Join(dir, "Sample Video.en.vtt"), filepath.Join(dir, "Sample Video.zh.vtt")},
		Success:    true,
		StartedAt:  start,
		FinishedAt: end,
	}
}
