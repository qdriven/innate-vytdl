// Package ytdlpbin locates or provisions a yt-dlp executable.
//
// Resolve order:
//  1. Explicit path (flag / config / YT_DL_BIN)
//  2. yt-dlp or youtube-dl on PATH
//  3. Binary embedded at build time (-tags embed_ytdlp)
//  4. Cached copy under the user cache dir
//  5. Download from GitHub Releases into the cache
package ytdlpbin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Resolve returns a usable yt-dlp binary path.
// explicit may be empty; when set it must exist (LookPath for bare names).
func Resolve(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return resolveExplicit(path)
	}
	if path := strings.TrimSpace(os.Getenv("YT_DL_BIN")); path != "" {
		return resolveExplicit(path)
	}
	for _, name := range []string{"yt-dlp", "youtube-dl"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	if path, err := ensureEmbedded(); err == nil && path != "" {
		return path, nil
	}
	if path, err := ensureCached(true); err == nil && path != "" {
		return path, nil
	}
	return "", fmt.Errorf("%w\n%s", ErrNotFound, installHint())
}

func resolveExplicit(path string) (string, error) {
	if filepath.IsAbs(path) || strings.ContainsAny(path, `/\`) {
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			return "", fmt.Errorf("yt-dlp binary not found at %q: %w", path, err)
		}
		return path, nil
	}
	found, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("yt-dlp binary not found: %q: %w", path, err)
	}
	return found, nil
}

// CacheDir returns ~/.cache/vYtDL (or OS equivalent).
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "vYtDL")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func cachedBinaryPath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name = "yt-dlp.exe"
	}
	return filepath.Join(dir, name), nil
}

func ensureCached(downloadIfMissing bool) (string, error) {
	path, err := cachedBinaryPath()
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() && st.Size() > 0 {
		return path, nil
	}
	if !downloadIfMissing {
		return "", ErrNotFound
	}
	if err := DownloadTo(path); err != nil {
		return "", err
	}
	return path, nil
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install yt-dlp (brew install yt-dlp), set --yt-dlp-bin, or allow auto-download from GitHub."
	case "windows":
		return "Install yt-dlp (winget install yt-dlp.yt-dlp), set --yt-dlp-bin, or allow auto-download from GitHub."
	default:
		return "Install yt-dlp (pipx install yt-dlp), set --yt-dlp-bin, or allow auto-download from GitHub."
	}
}
