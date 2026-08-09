package ytdlpbin

import (
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
