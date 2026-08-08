package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"strings"

	"github.com/spf13/cobra"

	"github.com/innate/yt-dl/internal/config"
	"github.com/innate/yt-dl/internal/downloader"
	"github.com/innate/yt-dl/internal/record"
	"github.com/innate/yt-dl/internal/tui"
)

var (
	flagFormat      string
	flagQuality     string
	flagStartTime   string
	flagEndTime     string
	flagOutputDir   string
	flagSubLangs    string
	flagNoSubs      bool
	flagNoAutoSubs  bool
	flagPlaylist    bool
	flagLogFormat   string
	flagRecordFile  string
	flagMappingFile string
	flagNoTUI       bool
	flagYTDLPBin    string
	flagProxy       string
	flagCookiesFile string
	flagCookiesFrom string
	flagUserAgent   string
	flagExtractor   string
	flagRetries     string
	flagTimeout     string
	flagForceIPv4   bool
	flagResetState  bool
	flagConcurrency int
)

func init() {
	dl := downloadCmd
	cfg := config.Load()

	dl.Flags().StringVarP(&flagFormat, "format", "f", "mp4",
		"Output container format: mp4, webm, mkv, …")
	dl.Flags().StringVarP(&flagQuality, "quality", "q", "",
		"Video quality: 720, 1080, 2160, … (empty = best)")
	dl.Flags().StringVar(&flagStartTime, "start", "",
		"Clip start time (HH:MM:SS or seconds)")
	dl.Flags().StringVar(&flagEndTime, "end", "",
		"Clip end time (HH:MM:SS or seconds)")
	dl.Flags().StringVarP(&flagOutputDir, "output", "o", ".",
		"Output directory (default: current directory)")
	dl.Flags().StringVar(&flagSubLangs, "sub-langs", "en,zh",
		"Comma-separated subtitle languages to download")
	dl.Flags().BoolVar(&flagNoSubs, "no-subs", false,
		"Disable subtitle download")
	dl.Flags().BoolVar(&flagNoAutoSubs, "no-auto-subs", false,
		"Disable auto-generated subtitle download")
	dl.Flags().BoolVarP(&flagPlaylist, "playlist", "p", false,
		"Treat URL as a playlist / collection")
	dl.Flags().StringVar(&flagLogFormat, "log-format", "json",
		"Record / mapping file format: json or csv")
	dl.Flags().StringVar(&flagRecordFile, "record-file", "download_record",
		"Base name (no extension) for the download log file")
	dl.Flags().StringVar(&flagMappingFile, "mapping-file", "subtitle_mapping",
		"Base name (no extension) for the subtitle-video mapping file")
	dl.Flags().BoolVar(&flagNoTUI, "no-tui", false,
		"Disable TUI; print plain progress to stdout")
	dl.Flags().StringVar(&flagYTDLPBin, "yt-dlp-bin", cfg.YTDLPBin,
		"Path to the yt-dlp/youtube-dl binary")
	dl.Flags().StringVar(&flagProxy, "proxy", "",
		"HTTP/HTTPS/SOCKS proxy URL passed through to yt-dlp")
	dl.Flags().StringVar(&flagCookiesFile, "cookies", "",
		"Netscape-format cookies file passed through to yt-dlp")
	dl.Flags().StringVar(&flagCookiesFrom, "cookies-from-browser", "",
		"Browser profile selector passed through to yt-dlp")
	dl.Flags().StringVar(&flagUserAgent, "user-agent", "",
		"Custom User-Agent string passed through to yt-dlp")
	dl.Flags().StringVar(&flagExtractor, "extractor-args", "",
		"Extractor args passed through to yt-dlp, e.g. youtube:player_client=web,android")
	dl.Flags().StringVar(&flagRetries, "retries", "10",
		"Retry count passed through to yt-dlp")
	dl.Flags().StringVar(&flagTimeout, "socket-timeout", "30",
		"Socket timeout in seconds passed through to yt-dlp")
	dl.Flags().BoolVar(&flagForceIPv4, "force-ipv4", false,
		"Force IPv4 for yt-dlp network requests")
	dl.Flags().BoolVar(&flagResetState, "reset-playlist-state", false,
		"Discard saved playlist state and start the playlist from the beginning")
	dl.Flags().IntVarP(&flagConcurrency, "concurrency", "j", 1,
		"Maximum number of concurrent downloads (1 = sequential)")

	rootCmd.AddCommand(dl)
}

var downloadCmd = &cobra.Command{
	Use:     "download [flags] <url> [url…]",
	Aliases: []string{"dl", "get"},
	Short:   "Download video(s) from any yt-dlp-supported site",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runDownload,
}

func runDownload(cmd *cobra.Command, args []string) error {
	logFormat := strings.ToLower(strings.TrimSpace(flagLogFormat))
	if logFormat != "json" && logFormat != "csv" {
		return fmt.Errorf("unsupported log format %q: use json or csv", flagLogFormat)
	}
	if err := validateYTDLPAvailability(flagYTDLPBin); err != nil {
		return err
	}

	// Resolve output directory
	outDir := flagOutputDir
	if outDir == "" {
		outDir = "."
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("cannot create output directory %q: %w", outDir, err)
	}

	langs := []string{"en", "zh"}
	if trimmed := strings.TrimSpace(flagSubLangs); trimmed != "" {
		parts := strings.Split(trimmed, ",")
		langs = langs[:0]
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				langs = append(langs, part)
			}
		}
	}

	opts := downloader.Options{
		Format:             flagFormat,
		Quality:            flagQuality,
		StartTime:          flagStartTime,
		EndTime:            flagEndTime,
		OutputDir:          outDir,
		SubtitleLangs:      langs,
		WriteSubtitles:     !flagNoSubs,
		WriteAutoSubs:      !flagNoAutoSubs,
		IsPlaylist:         flagPlaylist,
		LogFormat:          logFormat,
		RecordFile:         flagRecordFile,
		MappingFile:        flagMappingFile,
		YTDLPBin:           flagYTDLPBin,
		Proxy:              flagProxy,
		CookiesFile:        flagCookiesFile,
		CookiesFromBrowser: flagCookiesFrom,
		UserAgent:          flagUserAgent,
		ExtractorArgs:      flagExtractor,
		Retries:            flagRetries,
		SocketTimeout:      flagTimeout,
		ForceIPv4:          flagForceIPv4,
		ResetPlaylistState: flagResetState,
	}

	mgr := record.NewManager(logFormat, flagRecordFile, flagMappingFile, outDir)

	// Create progress channel
	progressCh := make(chan downloader.ProgressUpdate, 100)

	// Start TUI or plain logger in a goroutine
	tuiDone := make(chan error, 1)
	if !flagNoTUI {
		go func() {
			tuiDone <- tui.Run(progressCh)
		}()
	} else {
		go func() {
			for upd := range progressCh {
				switch upd.Status {
				case "done":
					fmt.Printf("[done]  %s\n", upd.Title)
				case "error":
					fmt.Printf("[error] %s: %s\n", upd.Title, upd.Error)
				case "downloading":
					fmt.Printf("[%.1f%%] %s  %s  ETA %s\n",
						upd.Percent, upd.Title, upd.Speed, upd.ETA)
				default:
					fmt.Printf("[%s] %s\n", upd.Status, upd.Title)
				}
			}
			tuiDone <- nil
		}()
	}

	dl := downloader.New(opts, progressCh)

	if flagConcurrency < 1 {
		flagConcurrency = 1
	}

	concurrency := flagConcurrency

	// --- Concurrent dispatch ---
	type workItem struct {
		url      string
		playlist bool
	}

	queue := make(chan workItem, len(args))
	for _, url := range args {
		queue <- workItem{url: url, playlist: flagPlaylist}
	}
	close(queue)

	var (
		allResults   []downloader.DownloadResult
		resultsMu    sync.Mutex
		wg           sync.WaitGroup
		sem          = make(chan struct{}, concurrency)
	)

	// If concurrency == 1, just do sequential (avoids goroutine overhead)
	if concurrency == 1 {
		for item := range queue {
			if item.playlist {
				results := dl.DownloadPlaylist(item.url)
				resultsMu.Lock()
				allResults = append(allResults, results...)
				resultsMu.Unlock()
			} else {
				result := dl.DownloadSingle(item.url)
				resultsMu.Lock()
				allResults = append(allResults, result)
				resultsMu.Unlock()
			}
		}
	} else {
		for item := range queue {
			wg.Add(1)
			go func(wi workItem) {
				defer wg.Done()
				sem <- struct{}{}        // acquire
				defer func() { <-sem }() // release

				if wi.playlist {
					results := dl.DownloadPlaylist(wi.url)
					resultsMu.Lock()
					allResults = append(allResults, results...)
					resultsMu.Unlock()
				} else {
					result := dl.DownloadSingle(wi.url)
					resultsMu.Lock()
					allResults = append(allResults, result)
					resultsMu.Unlock()
				}
			}(item)
		}
		wg.Wait()
	}

	// Signal TUI that all downloads are complete
	close(progressCh)
	if err := <-tuiDone; err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}

	// Write records
	for _, r := range allResults {
		mgr.Add(r)
	}
	if err := mgr.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write records: %v\n", err)
	} else {
		fmt.Printf("\nDownload log   : %s\n", mgr.RecordPath())
		fmt.Printf("Subtitle map   : %s\n", mgr.MappingPath())
	}

	if flagPlaylist && len(allResults) == 0 {
		fmt.Println("\nNo pending playlist items. Existing playlist state already marks all items as completed.")
		return nil
	}

	// Summary
	ok, fail := 0, 0
	for _, r := range allResults {
		if r.Success {
			ok++
		} else {
			fail++
		}
	}
	fmt.Printf("\nCompleted: %d succeeded, %d failed.\n", ok, fail)
	if fail > 0 {
		return fmt.Errorf("%d download(s) failed — see %s for details", fail, mgr.RecordPath())
	}
	return nil
}

func validateYTDLPAvailability(bin string) error {
	candidate := strings.TrimSpace(bin)
	if candidate == "" {
		candidate = "yt-dlp"
	}
	if _, err := exec.LookPath(candidate); err == nil {
		return nil
	}

	hint := "Install yt-dlp and make sure it is in PATH.\n- pipx install yt-dlp\n- or pip install -U yt-dlp"
	switch runtime.GOOS {
	case "darwin":
		hint = "Install yt-dlp and make sure it is in PATH.\n- brew install yt-dlp\n- or pipx install yt-dlp"
	case "windows":
		hint = "Install yt-dlp and make sure it is in PATH.\n- winget install yt-dlp.yt-dlp\n- or choco install yt-dlp"
	}

	return fmt.Errorf("yt-dlp binary not found: %q\n%s\nYou can also set --yt-dlp-bin or vYtDL/config.json with the full path", candidate, hint)
}