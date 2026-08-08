# vYtDL CLI

Go CLI / TUI wrapper around [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading videos from **1800+ sites** (YouTube, Bilibili, TikTok, X/Twitter, Instagram, Vimeo, Twitch, 小红书, …).

This repository is the **standalone CLI**. Desktop / Web live in the `innate-keepthem` monorepo.

## Requirements

- Go **1.24+**
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) on `PATH` (or set path in `config.json`)
- Optional: FFmpeg (time-range clips / merge)

## Build

```bash
git clone <this-repo> vYtDL
cd vYtDL
go build -o vYtDL .

# Cross-compile
./scripts/build.sh
```

Copy `config.example.json` → `config.json` if you need a fixed yt-dlp path:

```json
{ "yt_dlp_bin": "/opt/homebrew/bin/yt-dlp" }
```

Empty `yt_dlp_bin` means: use `PATH` (`yt-dlp` / `youtube-dl`).

## Quick usage

```bash
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

./vYtDL download --no-tui -o ./downloads --playlist \
  "https://www.bilibili.com/video/BVxxxxxx"

./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"
```

Full guide: [USAGE.md](./USAGE.md) (multi-platform examples + comparison vs raw yt-dlp).

## vs raw yt-dlp

Same engine. vYtDL simplifies everyday flags (`-q 1080`, default EN/ZH subs, `-o` dir, playlist resume, TUI, download records). Niche yt-dlp flags → use yt-dlp directly.

## Agent skill

Project skill for Cursor / other agents:

```
.agents/skills/vytdl-cli/SKILL.md
```

Symlink or copy into `~/.cursor/skills/vytdl-cli` to use from any workspace.

## Layout

```
cmd/                 # Cobra commands (download, analyze)
internal/downloader/ # yt-dlp spawn + progress
internal/tui/        # Bubble Tea UI
internal/record/     # download_record / subtitle_mapping
internal/vtt/        # VTT parse
scripts/             # build + shell helpers
```

## License

MIT — see [LICENSE](./LICENSE).
