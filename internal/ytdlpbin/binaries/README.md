# Embedded yt-dlp placeholder
#
# Default builds do NOT embed yt-dlp (`embed_disabled.go`).
# For a single-file release that embeds yt-dlp:
#
#   ./scripts/fetch-ytdlp.sh --embed
#   go build -tags embed_ytdlp -o vYtDL .
#
# That writes yt-dlp.bin here (gitignored). Do not commit real binaries.
