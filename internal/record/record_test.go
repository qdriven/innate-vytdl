package record

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/innate/yt-dl/internal/downloader"
)

func TestManagerFlushJSONIncludesSubtitles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "download_record.json")
	mgr, err := NewManager("json", path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.Add(sampleResult(dir))
	if err := mgr.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	cat := readCatalogFile(t, path)
	if cat.Version != 1 || len(cat.Records) != 1 {
		t.Fatalf("catalog = %#v", cat)
	}
	if !cat.Records[0].Success || len(cat.Records[0].Subtitles) != 2 {
		t.Fatalf("entry = %#v", cat.Records[0])
	}
	if _, err := os.Stat(filepath.Join(dir, "subtitle_mapping.json")); !os.IsNotExist(err) {
		t.Fatalf("subtitle mapping file should not be written, stat err=%v", err)
	}
}

func TestManagerFlushCSVRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "download_record.csv")
	mgr, err := NewManager("csv", path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.Add(sampleResult(dir))
	if err := mgr.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	fh, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer fh.Close()
	rows, err := csv.NewReader(fh).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 2 || rows[1][6] != "true" || !strings.Contains(rows[1][5], "Sample Video.en.vtt") {
		t.Fatalf("rows = %#v", rows)
	}

	again, err := NewManager("csv", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := again.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || len(got[0].Subtitles) != 2 || !got[0].Success {
		t.Fatalf("reloaded = %#v", got)
	}
}

func TestManagerAppendsOnRerun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "download_record.json")
	first, err := NewManager("json", path)
	if err != nil {
		t.Fatalf("first manager: %v", err)
	}
	first.Add(sampleResult(dir))
	if err := first.Flush(); err != nil {
		t.Fatalf("first flush: %v", err)
	}

	second, err := NewManager("json", path)
	if err != nil {
		t.Fatalf("second manager: %v", err)
	}
	next := sampleResult(dir)
	next.VideoID = "def456"
	next.Title = "Second Video"
	next.StartedAt = next.StartedAt.Add(time.Minute)
	next.FinishedAt = next.FinishedAt.Add(time.Minute)
	second.Add(next)
	if err := second.Flush(); err != nil {
		t.Fatalf("second flush: %v", err)
	}

	cat := readCatalogFile(t, path)
	if len(cat.Records) != 2 {
		t.Fatalf("expected 2 records, got %#v", cat.Records)
	}
}

func TestImportLegacyPairThenDeleteSources(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	home := t.TempDir()
	writeLegacyPair(t, project)

	path := filepath.Join(home, "download_record.json")
	mgr, err := NewManager("json", path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	n, err := mgr.ImportLegacyDir(project)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 2 {
		t.Fatalf("imported %d, want 2", n)
	}

	cat := readCatalogFile(t, path)
	if len(cat.Records) != 2 {
		t.Fatalf("catalog len = %d", len(cat.Records))
	}
	if len(cat.Records[1].Subtitles) != 2 || cat.Records[1].Subtitles[0] == "" {
		t.Fatalf("subtitles not merged: %#v", cat.Records[1])
	}
	if _, err := os.Stat(filepath.Join(project, "download_record.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy record still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "subtitle_mapping.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy mapping still present: %v", err)
	}

	again, err := mgr.ImportLegacyDir(project)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if again != 0 {
		t.Fatalf("second import added %d", again)
	}
}

func TestImportLegacyCSVWithoutSubtitleColumn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recordPath := filepath.Join(dir, "download_record.csv")
	mappingPath := filepath.Join(dir, "subtitle_mapping.csv")
	if err := os.WriteFile(recordPath, []byte("video_id,title,url,output_dir,filename,success,error,started_at,finished_at,duration\nabc123,Sample,https://example.com/v,.,Sample.mp4,true,,2026-03-18T10:00:00Z,2026-03-18T10:00:45Z,45s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mappingPath, []byte("video_id,title,video_file,subtitles\nabc123,Sample,Sample.mp4,Sample.en.vtt|Sample.zh.vtt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "download_record.json")
	mgr, err := NewManager("json", dest)
	if err != nil {
		t.Fatal(err)
	}
	n, err := mgr.ImportLegacyDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("imported %d", n)
	}
	cat := readCatalogFile(t, dest)
	if len(cat.Records) != 1 || len(cat.Records[0].Subtitles) != 2 || !cat.Records[0].Success {
		t.Fatalf("entry = %#v", cat.Records)
	}
}

func TestResolvePathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("VYTDL_HOME", home)
	if got := ResolvePath("json", ""); got != filepath.Join(home, "download_record.json") {
		t.Fatalf("default path = %s", got)
	}
	if got := ResolvePath("csv", "notes"); got != filepath.Join(home, "notes.csv") {
		t.Fatalf("bare name = %s", got)
	}
	abs := filepath.Join(home, "custom.json")
	if got := ResolvePath("json", abs); got != abs {
		t.Fatalf("absolute path = %s", got)
	}
}

func readCatalogFile(t *testing.T, path string) Catalog {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("unmarshal catalog: %v\n%s", err, data)
	}
	return cat
}

func writeLegacyPair(t *testing.T, dir string) {
	t.Helper()
	records := []DownloadRecord{
		{
			URL:        "https://example.com/fail",
			OutputDir:  dir,
			Success:    false,
			Error:      "boom",
			StartedAt:  time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 3, 18, 9, 0, 5, 0, time.UTC),
			Duration:   "5s",
		},
		{
			VideoID:    "abc123",
			Title:      "Sample Video",
			URL:        "https://example.com/watch?v=abc123",
			OutputDir:  dir,
			Filename:   "Sample Video.mp4",
			Success:    true,
			StartedAt:  time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 3, 18, 10, 0, 45, 0, time.UTC),
			Duration:   "45s",
		},
	}
	maps := []subtitleMapping{
		{},
		{
			VideoID:   "abc123",
			Title:     "Sample Video",
			VideoFile: "Sample Video.mp4",
			Subtitles: []string{"Sample Video.en.vtt", "Sample Video.zh.vtt"},
		},
	}
	writeJSON(t, filepath.Join(dir, "download_record.json"), records)
	writeJSON(t, filepath.Join(dir, "subtitle_mapping.json"), maps)
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
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
