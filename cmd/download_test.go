package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBatchURLs(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "urls.txt")
	content := "" +
		"# comment line\n" +
		"https://example.com/a\n" +
		"\n" +
		"  https://example.com/b  \n" +
		"   # indented comment\n" +
		"https://example.com/c\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write urls file: %v", err)
	}

	urls, err := readBatchURLs(path)
	if err != nil {
		t.Fatalf("readBatchURLs: %v", err)
	}
	want := []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"}
	if len(urls) != len(want) {
		t.Fatalf("got %d URLs %v, want %d", len(urls), urls, len(want))
	}
	for i, u := range want {
		if urls[i] != u {
			t.Errorf("urls[%d] = %q, want %q", i, urls[i], u)
		}
	}
}

func TestReadBatchURLsMissingFile(t *testing.T) {
	if _, err := readBatchURLs(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("expected error for missing batch file")
	}
}

func TestReadBatchURLsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, []byte("# only comments\n\n"), 0o644); err != nil {
		t.Fatalf("write urls file: %v", err)
	}
	urls, err := readBatchURLs(path)
	if err != nil {
		t.Fatalf("readBatchURLs: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected 0 URLs, got %v", urls)
	}
}
