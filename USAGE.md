# vYtDL Usage

## Quick Start

Build the binary:

```bash
cd vYtDL
go build -o vYtDL .
```

The default `yt-dlp` binary path is configured in `vYtDL/config.json`:

```json
{
  "yt_dlp_bin": "/Applications/ServBay/package/python/3.14/3.14.0b1/Python.framework/Versions/3.14/bin/yt-dlp"
}
```

If your local `yt-dlp` is somewhere else, edit `config.json`. You only need `--yt-dlp-bin` when you want to override that config for one command.

Common `yt_dlp_bin` values:

- macOS: `/opt/homebrew/bin/yt-dlp` or `/usr/local/bin/yt-dlp`
- Linux: `/usr/bin/yt-dlp` or `/usr/local/bin/yt-dlp`
- Windows 11: `C:\\Users\\<you>\\AppData\\Local\\Microsoft\\WinGet\\Links\\yt-dlp.exe`

You can also load config from a custom file path:

```bash
VYTDL_CONFIG=/absolute/path/to/config.json ./vYtDL download --no-tui "VIDEO_URL"
```

Before any download starts, the CLI resolves yt-dlp automatically (see **Bundled yt-dlp** below).

## Bundled yt-dlp

yt-dlp is still a separate executable (cannot be statically linked into Go). vYtDL provisions it in this order:

1. `--yt-dlp-bin` / `config.json` / `YT_DL_BIN`
2. `yt-dlp` / `youtube-dl` on `PATH`
3. **Embedded** binary (only if built with `-tags embed_ytdlp`)
4. Cache under `~/Library/Caches/vYtDL` (macOS) or `~/.cache/vYtDL`
5. **Auto-download** from GitHub Releases into that cache

```bash
# Force download/update into cache
./vYtDL download --install-yt-dlp

# Normal download — uses PATH or auto-downloads if missing
./vYtDL download --no-tui -o ./downloads "URL"

# Mirror (slow GitHub): full asset URL or prefix ending with /
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

### Single-file release (embed)

```bash
./scripts/fetch-ytdlp.sh --embed          # writes internal/ytdlpbin/binaries/yt-dlp.bin
go build -tags embed_ytdlp -o vYtDL .

# Cross-compile all platforms with embed:
EMBED=1 ./scripts/build.sh
```

Embedded builds extract to the cache on first run (no write to the app binary itself).

## Supported platforms

The CLI does **not** whitelist domains. Any URL yt-dlp can extract works (1800+ sites).

### Multi-platform examples

```bash
# YouTube — 1080p + EN/ZH 字幕（默认开字幕）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Bilibili — 合集/多 P 用 --playlist
./vYtDL download --no-tui -o ./downloads -q 1080 --playlist \
  "https://www.bilibili.com/video/BVxxxxxx"

# TikTok — 多数情况需要 Cookie
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"

# X / Twitter
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://x.com/user/status/123"

# 小红书
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.xiaohongshu.com/explore/xxxxxx"

# Instagram Reels / 帖子
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.instagram.com/reel/xxxxxx/"

# Vimeo
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://vimeo.com/123456789"

# Twitch VOD（直播回放）
./vYtDL download --no-tui -o ./downloads \
  "https://www.twitch.tv/videos/123456789"

# 只下某一时间段（任意支持分段的站点，需 FFmpeg）
./vYtDL download --no-tui -o ./downloads \
  --start 00:01:00 --end 00:02:30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# 国内网络：加代理
./vYtDL download --no-tui -o ./downloads \
  --proxy "http://127.0.0.1:7890" \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

List extractors on your machine:

```bash
yt-dlp --list-extractors
```

See also: `docs/suggestions/supported-platforms.md`.

## vYtDL vs 原始 yt-dlp：主要差别与简化

vYtDL **不是**另一套下载引擎，而是对 `yt-dlp` 的薄封装：最终仍 spawn yt-dlp。差别在「默认值、参数简化、工作流」。

| 维度 | 原始 yt-dlp | vYtDL CLI |
|------|-------------|-----------|
| 画质 | `-f "bestvideo[height<=1080]+bestaudio/..."` 自己写 | `-q 1080` |
| 容器 | `--merge-output-format mp4` | `-f mp4`（默认 mp4） |
| 字幕 | 多条 `--write-subs --write-auto-subs --sub-langs ...` | 默认开 EN+ZH；`--no-subs` 关掉 |
| 输出路径 | `-o "dir/%(title)s.%(ext)s"` | `-o ./dir`（自动套模板） |
| 时间片段 | `--download-sections "*1:00-2:30" --force-keyframes-at-cuts` | `--start` / `--end` |
| 播放列表 | 自己管目录与失败重试 | `--playlist`：按标题建子目录 + **断点续传** |
| 进度 | 文本滚动 | 默认 **TUI**；`--no-tui` 为纯文本 |
| 记录 | 自己记日志 | 自动写 `download_record` + `subtitle_mapping`（json/csv） |
| Cookie/代理 | 原样传 | 同名透传：`--cookies`、`--cookies-from-browser`、`--proxy` |
| 并发 | 自己脚本循环 | `-j N` 多 URL 并发 |
| 能力上限 | 全部 yt-dlp 旗标 | **常用子集**；冷门旗标用原生 yt-dlp |

### 等价对照（同一意图）

```bash
# 原生 yt-dlp
yt-dlp -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]" \
  --merge-output-format mp4 \
  --write-subs --write-auto-subs --sub-langs en,zh \
  -o "./downloads/%(title)s.%(ext)s" \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# vYtDL（同一件事）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"
```

**一句话**：站点覆盖 ≈ yt-dlp；省事的是预设（画质/字幕/输出/列表续传/TUI/下载记录），不是多出一个下载器。

## Single Video

Download one video into the current directory:

```bash
./vYtDL download --no-tui "https://www.youtube.com/watch?v=VIDEO_ID"
```

Download one video to a target folder with 1080p quality:

```bash
./vYtDL download --no-tui \
  --output ./downloads \
  --quality 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

Download a clip from a time range:

```bash
./vYtDL download --no-tui \
  --start 00:01:00 \
  --end 00:02:30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

## Collection Download

Download a full YouTube playlist or collection:

```bash
./vYtDL download --no-tui \
  --playlist \
  --output ./downloads \
  "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C"
```

Collection download with custom quality and CSV logs:

```bash
./vYtDL download --no-tui \
  --playlist \
  --quality 720 \
  --log-format csv \
  --output ./downloads \
  "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C"
```

What happens:

- `./downloads` is the root output directory.
- The playlist title becomes a subdirectory under `./downloads`.
- Each video is downloaded one by one into that playlist directory.
- `download_record.json` or `download_record.csv` is written in the output root.
- `subtitle_mapping.json` or `subtitle_mapping.csv` is written in the output root.

## Resume Solution For Playlist Downloads

Playlist resume is now built in.

The implementation works like this:

- vYtDL fetches the full playlist entry list first.
- It creates a state file named `.playlist_state.json` inside the playlist directory.
- Each video is tracked with a status: `pending`, `running`, `succeeded`, or `failed`.
- Downloads are executed one by one.
- After each item finishes, the state file is updated immediately.
- On the next run with the same playlist URL and output directory, already successful items are skipped and only unfinished or failed items are retried.

Resume example:

```bash
./vYtDL download --no-tui \
  --playlist \
  --output ./downloads \
  "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C"
```

If the run stops halfway, just run the exact same command again. vYtDL will resume from the remaining failed or unfinished items.

State file example location:

```text
./downloads/My Playlist/.playlist_state.json
```

Start the playlist from scratch and ignore the saved state:

```bash
./vYtDL download --no-tui \
  --playlist \
  --reset-playlist-state \
  --output ./downloads \
  "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C"
```

## Shell Scripts

Single video:

```bash
./scripts/download_video.sh "https://www.youtube.com/watch?v=VIDEO_ID" ./downloads
```

Single video with custom quality:

```bash
./scripts/download_video.sh "https://www.youtube.com/watch?v=VIDEO_ID" ./downloads 720
```

Collection:

```bash
./scripts/download_collection.sh "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C" ./downloads
```

Collection with custom quality:

```bash
./scripts/download_collection.sh "https://www.youtube.com/playlist?list=PL2C4A8A7A6F3A5D3C" ./downloads 720
```

The shell script resume behavior is the same as the CLI. Re-run the same script command and it will continue from unfinished playlist items.
Both `download_video.sh` and `download_collection.sh` also check for `yt-dlp`/`youtube-dl` in PATH before running.

## Output Files

Each run writes two main files to the chosen output directory:

- `download_record.json` or `download_record.csv`
- `subtitle_mapping.json` or `subtitle_mapping.csv`

For playlist runs, vYtDL also writes:

- `.playlist_state.json` inside the playlist directory

The download record includes:

- whether the download succeeded
- failure reason when it did not
- source URL
- output path
- timestamps and duration

The subtitle mapping includes:

- video id
- video title
- video file path
- subtitle file paths

The playlist state file includes:

- full playlist queue
- per-video status
- attempt count
- last error
- last finished filename

## Concurrent Downloads

Use the `--concurrency` / `-j` flag to download multiple URLs in parallel:

```bash
# Two videos at once
./vYtDL download --no-tui -j 2 \
  "URL1" "URL2" "URL3"

# Batch with configurable concurrency
./vYtDL download --no-tui --concurrency 3 \
  --output ./downloads \
  "URL1" "URL2" "URL3" "URL4"
```

- Default: `-j 1` (sequential, same as old behavior)
- Internally uses a worker pool with semaphore + WaitGroup
- Results and records are thread-safe
- Higher concurrency = more YouTube requests = higher chance of triggering anti-bot measures

## Subtitle Analysis

Once you have downloaded `.vtt` subtitle files, you can extract plain text with the `analyze` command:

```bash
# Plain text extraction
./vYtDL analyze --mode text video.zh.vtt

# Write to file
./vYtDL analyze --mode text --output transcript.txt video.en.vtt

# Pipe from stdin
./vYtDL analyze < video.vtt
```

Aliases: `an`, `ana`

The parser handles both simple manual captions and YouTube auto-generated captions (with `<c>` word-timing tags). More analysis modes (`summary`, `keypoints`) are planned — see `tasks/vtt-analysis-spec.md`.

## Recovery Options

If YouTube blocks anonymous extraction, use the recovery flags from `help.md`:

```bash
./vYtDL download --no-tui \
  --cookies-from-browser chrome \
  --extractor-args "youtube:player_client=web,android" \
  --force-ipv4 \
  --socket-timeout 30 \
  --retries 10 \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

**YouTube `n` challenge**: If you see `Sign in to confirm you're not a bot` even with cookies, YouTube's JavaScript challenge requires a JS runtime:

```bash
# Use Node.js (if available) via yt-dlp directly:
yt-dlp --cookies-from-browser chrome --js-runtimes node "URL"

# Or install deno:
brew install deno
```

The vYtDL CLI does not yet pass `--js-runtimes` through; use yt-dlp directly for now when the `n` challenge appears.

Supported passthrough flags:

- `--yt-dlp-bin`
- `--proxy`
- `--cookies`
- `--cookies-from-browser`
- `--user-agent`
- `--extractor-args`
- `--retries`
- `--socket-timeout`
- `--force-ipv4`

## Known Fix

The original failure log used an escaped URL:

```text
https://www.youtube.com/watch\?v\=EBWTRvjZ1dw
```

The downloader now normalizes that input automatically to:

```text
https://www.youtube.com/watch?v=EBWTRvjZ1dw
```
