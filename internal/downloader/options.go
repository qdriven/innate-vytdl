package downloader

// Options holds download configuration for a single video or playlist.
type Options struct {
	// URL is the target video or playlist URL.
	URL string

	// Format selects the container format, e.g. "mp4", "webm". Empty = best.
	Format string

	// Quality selects the video quality, e.g. "720", "1080". Empty = best.
	Quality string

	// StartTime / EndTime define a time range (HH:MM:SS or seconds).
	// Both empty means download the full video.
	StartTime string
	EndTime   string

	// OutputDir is the destination directory. Defaults to the current directory.
	OutputDir string

	// SubtitleLangs lists subtitle languages to download. Default: ["en", "zh"].
	SubtitleLangs []string

	// WriteSubtitles enables subtitle download.
	WriteSubtitles bool

	// WriteAutoSubs downloads auto-generated subtitles when manual ones are missing.
	WriteAutoSubs bool

	// IsPlaylist indicates the URL points to a collection/playlist.
	// Each playlist gets its own sub-directory named after the playlist title.
	IsPlaylist bool

	// RecordFile is the path to the download-log file (.json or .csv).
	RecordFile string

	// MappingFile is the path to the subtitle-video mapping file (.json or .csv).
	MappingFile string

	// LogFormat selects "json" or "csv" for the record / mapping files.
	LogFormat string

	// YTDLPBin optionally overrides the downloader binary path.
	YTDLPBin string

	// Proxy configures an optional HTTP/SOCKS proxy.
	Proxy string

	// CookiesFile provides a Netscape-format cookies file to yt-dlp.
	CookiesFile string

	// CookiesFromBrowser loads cookies from a local browser profile.
	CookiesFromBrowser string

	// UserAgent overrides the default user agent for requests.
	UserAgent string

	// ExtractorArgs forwards extractor-specific options, e.g. youtube:player_client=web,android.
	ExtractorArgs string

	// Retries overrides yt-dlp retry count. Empty uses yt-dlp defaults.
	Retries string

	// SocketTimeout sets the network timeout in seconds.
	SocketTimeout string

	// ForceIPv4 forces yt-dlp to use IPv4.
	ForceIPv4 bool

	// ResetPlaylistState discards any saved playlist resume state before downloading.
	ResetPlaylistState bool
}

// DefaultOptions returns sane defaults.
func DefaultOptions() Options {
	return Options{
		Format:         "mp4",
		Quality:        "bestvideo+bestaudio",
		OutputDir:      ".",
		SubtitleLangs:  []string{"en", "zh"},
		WriteSubtitles: true,
		WriteAutoSubs:  true,
		LogFormat:      "json",
		RecordFile:     "download_record",
		MappingFile:    "subtitle_mapping",
		Retries:        "10",
		SocketTimeout:  "30",
	}
}
