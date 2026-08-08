package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "yt-dl",
	Short:        "A multi-site video downloader powered by yt-dlp",
	SilenceUsage: true,
	Long: `yt-dl (vYtDL) is a command-line / TUI tool for downloading videos via yt-dlp.

Supported sites follow yt-dlp (1800+ extractors): YouTube, Bilibili, TikTok,
X/Twitter, Instagram, Vimeo, Twitch, and many more. Pass any URL yt-dlp accepts.

Features:
  • Single video or full playlist download
  • Format selection (mp4, webm, …)
  • Quality selection (720p, 1080p, …)
  • Time-range clipping
  • Subtitle download (EN + ZH by default)
  • Cookie / proxy / extractor-args for restricted sites
  • Download log (JSON or CSV) tracking success / failure
  • Subtitle-video mapping file (JSON or CSV)
  • Interactive TUI with live progress bars
`,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
