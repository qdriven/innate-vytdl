# vYtDL CLI

Go CLI / TUI wrapper around [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading videos from **1800+ sites** (YouTube, Bilibili, TikTok, X/Twitter, Instagram, Vimeo, Twitch, 小红书, …).

The canonical source now lives **in the monorepo** at `services/vytdl/`
(refactor Sprint 1, Q1 decision); `qdriven/innate-vytdl` receives one-way sync
PRs from it. The module path remains `github.com/innate/yt-dl`.

Beyond the CLI/TUI, this module ships the **background service** (`vYtDL serve`)
that acts as the single backend for the web UI — see [USAGE](USAGE.md#background-service-serve).

## Requirements

- Go **1.26+**
- Optional: system [yt-dlp](https://github.com/yt-dlp/yt-dlp) (otherwise auto-download or embed)
- Optional: FFmpeg (time-range clips / merge)

## Build

```bash
# from the monorepo root (source lives in services/vytdl/)
cd services/vytdl
go build -o vYtDL .

# Cross-compile all platforms (macOS / Linux / Windows)
./scripts/build.sh
# → dist/vYtDL-darwin-arm64, …, dist/vYtDL-windows-amd64.exe, …

# Windows only (amd64 + arm64)
./scripts/build.sh          # all targets
ONLY=windows ./scripts/build.sh
# or: task build:windows
# or (PowerShell): .\scripts\build.ps1 -Only windows

# Single-file release with yt-dlp embedded
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
# or: EMBED=1 ONLY=windows ./scripts/build.sh
```

Copy `config.example.json` → `config.json` only if you need a fixed path:

```json
{ "yt_dlp_bin": "/opt/homebrew/bin/yt-dlp" }
```

Empty `yt_dlp_bin` means: PATH → embed → cache → GitHub download.

## Quick usage

```bash
# Install/update yt-dlp into local cache (no system install needed)
./vYtDL download --install-yt-dlp

./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

./vYtDL download --no-tui -o ./downloads --playlist \
  "https://www.bilibili.com/video/BVxxxxxx"

./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"
```

Slow GitHub downloads:

```bash
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

Full guide: [USAGE.md](./USAGE.md) (multi-platform examples + vs raw yt-dlp).

## vs raw yt-dlp

Same engine. vYtDL simplifies everyday flags (`-q 1080`, default EN/ZH subs, `-o` dir, playlist resume, TUI, download records) and can provision yt-dlp for you.

## Agent skill

```
.agents/skills/vytdl-cli/SKILL.md
```

```bash
mkdir -p ~/.cursor/skills
ln -sfn "$(pwd)/.agents/skills/vytdl-cli" ~/.cursor/skills/vytdl-cli
```

## Layout

```
cmd/                 # Cobra commands (download, analyze)
internal/downloader/ # yt-dlp spawn + progress
internal/ytdlpbin/   # PATH / embed / auto-download resolver
internal/tui/        # Bubble Tea UI
internal/record/     # download_record / subtitle_mapping
internal/vtt/        # VTT parse
scripts/             # build + fetch-ytdlp helpers
```

## License

Add a LICENSE file when publishing. Code follows the parent project conventions.
