package ytdlpbin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseAssetName(t *testing.T) {
	t.Parallel()
	name, err := ReleaseAssetName()
	if err != nil {
		t.Fatalf("ReleaseAssetName: %v", err)
	}
	if name == "" {
		t.Fatal("empty asset name")
	}
	switch runtime.GOOS {
	case "darwin":
		if name != "yt-dlp_macos" {
			t.Fatalf("got %q", name)
		}
	case "linux":
		if runtime.GOARCH == "amd64" && name != "yt-dlp_linux" {
			t.Fatalf("got %q", name)
		}
	case "windows":
		if runtime.GOARCH == "amd64" && name != "yt-dlp.exe" {
			t.Fatalf("got %q", name)
		}
	}
}

func TestDownloadURLLatest(t *testing.T) {
	t.Parallel()
	u, err := DownloadURL("latest")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "https://") {
		t.Fatalf("unexpected url %q", u)
	}
	asset, _ := ReleaseAssetName()
	if !strings.Contains(u, asset) {
		t.Fatalf("url %q missing asset %q", u, asset)
	}
}

func TestResolveExplicitMissing(t *testing.T) {
	t.Parallel()
	_, err := Resolve(filepath.Join(t.TempDir(), "no-such-yt-dlp"))
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestEmbeddedFlagMatchesBuild(t *testing.T) {
	t.Parallel()
	if Embedded() {
		t.Fatal("default build should not report Embedded()==true")
	}
}

func TestLocalBinaryPath(t *testing.T) {
	t.Parallel()
	got := localBinaryPath()
	want := "bin/yt-dlp"
	if runtime.GOOS == "windows" {
		want = `bin\yt-dlp.exe`
	}
	if got != want {
		t.Fatalf("localBinaryPath() = %q, want %q", got, want)
	}
}

func TestEnsureLocalBinMissingNoDownload(t *testing.T) {
	// Not parallel: chdir-based test.
	t.Chdir(t.TempDir())
	if _, err := ensureLocalBin(false); err == nil {
		t.Fatal("expected ErrNotFound for missing ./bin copy without download")
	}
}

func TestEnsureLocalBinUsesExistingCopy(t *testing.T) {
	// Not parallel: chdir-based test.
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("bin", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localBinaryPath(), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := ensureLocalBin(true)
	if err != nil {
		t.Fatalf("ensureLocalBin(true) with existing copy: %v", err)
	}
	if path != localBinaryPath() {
		t.Fatalf("got %q, want %q", path, localBinaryPath())
	}
}
