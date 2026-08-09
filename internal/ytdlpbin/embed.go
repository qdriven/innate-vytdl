//go:build embed_ytdlp

package ytdlpbin

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Place the platform binary at binaries/yt-dlp.bin before building with:
//
//	go build -tags embed_ytdlp
//
// scripts/fetch-ytdlp.sh --embed downloads the correct asset into that path.
//
//go:embed binaries/yt-dlp.bin
var embeddedYTDLP []byte

func ensureEmbedded() (string, error) {
	if len(embeddedYTDLP) == 0 {
		return "", fmt.Errorf("embedded yt-dlp is empty")
	}
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	name := "yt-dlp-embedded"
	if runtime.GOOS == "windows" {
		name = "yt-dlp-embedded.exe"
	}
	path := filepath.Join(dir, name)
	marker := path + ".size"
	want := fmt.Sprintf("%d", len(embeddedYTDLP))
	if data, err := os.ReadFile(marker); err == nil && string(data) == want {
		if st, err := os.Stat(path); err == nil && !st.IsDir() && st.Size() == int64(len(embeddedYTDLP)) {
			return path, nil
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, embeddedYTDLP, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Chmod(path, 0o755)
	_ = os.WriteFile(marker, []byte(want), 0o644)
	return path, nil
}

// Embedded returns whether this build includes an embedded yt-dlp binary.
func Embedded() bool {
	return len(embeddedYTDLP) > 0
}
