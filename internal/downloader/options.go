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

	// JSRuntimes selects JS runtimes for YouTube n-challenge solving, e.g. "node" or "deno".
	// Passed through to yt-dlp as --js-runtimes. Empty skips the flag (yt-dlp default: deno).
	JSRuntimes string

	// Retries overrides yt-dlp retry count. Empty uses yt-dlp defaults.
	Retries string

	// SocketTimeout sets the network timeout in seconds.
	SocketTimeout string

	// ForceIPv4 forces yt-dlp to use IPv4.
	ForceIPv4 bool

	// ResetPlaylistState discards any saved playlist resume state before downloading.
	ResetPlaylistState bool

	// FormatID is an exact yt-dlp format selector (Format Picker route).
	// When set it overrides Quality.
	FormatID string

	// RateLimit caps download speed, e.g. "500K" or "2M". Passed as -r.
	RateLimit string

	// ConcurrentFragments enables fragmented downloads with N threads.
	// 0 disables the flag (yt-dlp default).
	ConcurrentFragments int

	// EmbedThumbnail / EmbedMetadata / EmbedChapters enable post-processing.
	// EmbedThumbnail is skipped for webm containers (no cover support).
	EmbedThumbnail bool
	EmbedMetadata  bool
	EmbedChapters  bool

	// SponsorblockRemove strips sponsor segments (--sponsorblock-remove all).
	SponsorblockRemove bool

	// FilenameTemplate overrides the output template. Empty uses
	// "%(title)s.%(ext)s".
	FilenameTemplate string

	// POToken is forwarded as an "youtube:po_token=..." extractor arg.
	POToken string

	// ExtractorArgsValues adds one --extractor-args flag per entry.
	// ExtractorArgs (single string) is kept for CLI compatibility and is
	// appended after these.
	ExtractorArgsValues []string

	// ConfigLocation points at a user-provided yt-dlp config file
	// (--config-locations). Empty + IgnoreUserConfig emits --ignore-config.
	ConfigLocation string

	// IgnoreUserConfig forces --ignore-config so user-level yt-dlp configs
	// cannot alter behavior (service mode default).
	IgnoreUserConfig bool

	// WindowsFilenames forces cross-platform-safe filenames (--windows-filenames).
	WindowsFilenames bool

	// SleepSubtitles delays automatic subtitle downloads (--sleep-subtitles).
	// Empty skips the flag. Service mode uses "1" to avoid rate limiting.
	SleepSubtitles string

	// SectionSafeSelector swaps the format selector for a muxed-safe one when
	// clipping time ranges, and disables direct merge, so progress does not
	// stall (desktop 坑知识 #1).
	SectionSafeSelector bool
}

// SectionSafeSelector is the muxed-safe format selector used for time-range
// clips when no exact format_id is given.
const SectionSafeSelector = "best[protocol!*=dash][vcodec!=none][acodec!=none]/best[protocol!*=dash]/best"

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
