package ytdlpbin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrNotFound means no yt-dlp binary could be resolved or installed.
var ErrNotFound = errors.New("yt-dlp binary not found")

const (
	defaultReleaseTag = "latest"
	githubReleaseAPI  = "https://github.com/yt-dlp/yt-dlp/releases/download"
)

// ReleaseAssetName returns the official yt-dlp release asset for this platform.
func ReleaseAssetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "yt-dlp_macos", nil
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "yt-dlp_linux", nil
		case "arm64":
			return "yt-dlp_linux_aarch64", nil
		default:
			return "", fmt.Errorf("unsupported linux arch %q for bundled yt-dlp", runtime.GOARCH)
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "yt-dlp.exe", nil
		case "arm64":
			return "yt-dlp_arm64.exe", nil
		default:
			return "", fmt.Errorf("unsupported windows arch %q for bundled yt-dlp", runtime.GOARCH)
		}
	default:
		return "", fmt.Errorf("unsupported OS %q for bundled yt-dlp", runtime.GOOS)
	}
}

// DownloadURL builds the GitHub release download URL.
// tag may be "latest" or a version like "2026.07.04".
func DownloadURL(tag string) (string, error) {
	asset, err := ReleaseAssetName()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(tag) == "" || tag == "latest" {
		// /latest/download/ASSET redirects to the newest release asset
		return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + asset, nil
	}
	return fmt.Sprintf("%s/%s/%s", githubReleaseAPI, tag, asset), nil
}

// DownloadTo fetches yt-dlp into destPath (atomic write).
func DownloadTo(destPath string) error {
	url, err := resolveDownloadURL()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	tmp := destPath + ".tmp"
	_ = os.Remove(tmp)

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vYtDL/"+runtime.GOOS)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download yt-dlp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download yt-dlp: HTTP %d from %s", resp.StatusCode, url)
	}

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)
	n, copyErr := io.Copy(writer, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if n < 1024*100 {
		_ = os.Remove(tmp)
		return fmt.Errorf("download yt-dlp: file too small (%d bytes), likely not a binary", n)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	metaPath := destPath + ".sha256"
	_ = os.WriteFile(metaPath, []byte(sum+"\n"), 0o644)

	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(destPath, 0o755)
	return nil
}

func resolveDownloadURL() (string, error) {
	if mirror := strings.TrimSpace(os.Getenv("VYTDL_YTDLP_MIRROR")); mirror != "" {
		if strings.HasSuffix(mirror, "/") {
			asset, err := ReleaseAssetName()
			if err != nil {
				return "", err
			}
			return mirror + asset, nil
		}
		return mirror, nil
	}
	return DownloadURL(defaultReleaseTag)
}

// InstallOrUpdate downloads (or re-downloads) yt-dlp into the cache and returns its path.
func InstallOrUpdate() (string, error) {
	path, err := cachedBinaryPath()
	if err != nil {
		return "", err
	}
	if err := DownloadTo(path); err != nil {
		return "", err
	}
	return path, nil
}
