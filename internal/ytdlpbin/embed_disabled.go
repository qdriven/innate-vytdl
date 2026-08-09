//go:build !embed_ytdlp

package ytdlpbin

func ensureEmbedded() (string, error) {
	return "", ErrNotFound
}

// Embedded returns whether this build includes an embedded yt-dlp binary.
func Embedded() bool {
	return false
}
