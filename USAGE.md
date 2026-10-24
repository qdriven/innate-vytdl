# vYtDL Usage

Go CLI / TUI wrapper around [yt-dlp](https://github.com/yt-dlp/yt-dlp). vYtDL does **not** reimplement downloaders — it spawns yt-dlp, simplifies everyday flags, and adds playlist resume, TUI progress, and download records.

**Full guide for YouTube and Bilibili** is below. Other sites (TikTok, X, 小红书, …) use the same CLI; see [Multi-site tips](#other-sites).

---

## Quick start

```bash
cd vYtDL-standalone   # or tools/vytdl-cli in the monorepo
go build -o vYtDL .  # GOWORK=off if inside a go.work

./vYtDL download --install-yt-dlp   # optional: cache a fresh yt-dlp

# YouTube single video (1080p, EN+ZH subs by default)
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Bilibili multi-P / collection
./vYtDL download --no-tui -o ./downloads -q 1080 --playlist \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"
```

Defaults worth knowing:

| Default | Value |
|---------|--------|
| Container | `-f mp4` |
| Subtitles | EN + ZH, including auto-subs |
| JS runtime (YouTube) | `--js-runtimes node` |
| Concurrency | `-j 1` (sequential) |
| Progress | TUI (use `--no-tui` for plain text) |
| Records | `download_record.json` + `subtitle_mapping.json` |

---

## How it works (implementation)

```
./vYtDL download [flags] URL…
        │
        ▼
  cmd/download.go          parse flags → downloader.Options
        │
        ▼
  internal/ytdlpbin        resolve yt-dlp binary
        │
        ▼
  internal/downloader      buildArgs() → exec yt-dlp
        │                    parse --newline progress
        │                    optional playliststate resume
        ▼
  internal/record         append download_record + subtitle_mapping
  internal/tui (optional)  Bubble Tea progress
```

### Flag → yt-dlp mapping

| vYtDL flag | yt-dlp equivalent (approx.) |
|-----------|------------------------------|
| `-q 1080` | `-f "bestvideo[height<=1080]+bestaudio/best[height<=1080]"` |
| `-f mp4` | `--merge-output-format mp4` |
| `-o ./dir` | `-o "./dir/%(title)s.%(ext)s"` |
| (default subs) | `--write-subs --write-auto-subs --sub-langs en,zh` |
| `--start` / `--end` | `--download-sections "*START-END" --force-keyframes-at-cuts` |
| `--no-playlist` (default) | `--no-playlist` |
| `--playlist` | playlist mode + titled subdir + `.playlist_state.json` |
| `--cookies-from-browser` | same |
| `--cookies` | same |
| `--proxy` | same |
| `--extractor-args` | same |
| `--js-runtimes node` | same (default in vYtDL) |
| `--retries` / `--socket-timeout` / `--force-ipv4` | same |

Code path: `cmd/download.go` → `internal/downloader/options.go` → `buildArgs()` in `internal/downloader/downloader.go`.

### yt-dlp binary resolution

Order (`internal/ytdlpbin`):

1. `--yt-dlp-bin` / `config.json` `yt_dlp_bin` / `YT_DL_BIN`
2. `yt-dlp` / `youtube-dl` on `PATH`
3. Embedded binary (`-tags embed_ytdlp`)
4. Cache: `~/Library/Caches/vYtDL` (macOS) or `~/.cache/vYtDL`
5. GitHub Releases auto-download

```bash
./vYtDL download --install-yt-dlp
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

Empty `yt_dlp_bin` in config means “auto-resolve”. Copy `config.example.json` only when you need a fixed path:

```json
{ "yt_dlp_bin": "/opt/homebrew/bin/yt-dlp" }
```

---

## YouTube

### URL shapes

| Kind | Example |
|------|---------|
| Watch | `https://www.youtube.com/watch?v=VIDEO_ID` |
| Short URL | `https://youtu.be/VIDEO_ID` |
| Shorts | `https://www.youtube.com/shorts/VIDEO_ID` |
| Playlist | `https://www.youtube.com/playlist?list=PLxxxx` |
| Mix / radio | `list=RD…` — prefer default `--no-playlist` for a single watch URL |

Escaped shell URLs like `watch\?v\=ID` are normalized automatically.

### Single video

```bash
# Best quality under 1080p, EN+ZH subs, plain progress
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=N2GFDDZtGQY"

# Clip (needs FFmpeg)
./vYtDL download --no-tui -o ./downloads \
  --start 00:01:00 --end 00:02:30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# No subtitles
./vYtDL download --no-tui -o ./downloads --no-subs \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Only Chinese subs
./vYtDL download --no-tui -o ./downloads --sub-langs zh \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

Equivalent raw yt-dlp (what vYtDL approximates):

```bash
yt-dlp -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]" \
  --merge-output-format mp4 \
  --write-subs --write-auto-subs --sub-langs en,zh \
  --js-runtimes node \
  --cookies-from-browser chrome \
  -o "./downloads/%(title)s.%(ext)s" \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

### Playlist

```bash
./vYtDL download --no-tui --playlist -o ./downloads -q 720 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/playlist?list=PLxxxx"
```

What happens:

1. Fetch playlist metadata.
2. Create `./downloads/<Playlist Title>/`.
3. Write `.playlist_state.json` and download entry-by-entry.
4. Re-run the **same** command to resume; succeeded items are skipped.
5. `--reset-playlist-state` discards state and starts over.

```text
./downloads/My Playlist/
  .playlist_state.json
  Video One.mp4
  Video One.en.vtt
  …
./downloads/download_record.json
./downloads/subtitle_mapping.json
```

### YouTube anti-bot / 403 / “n challenge”

YouTube increasingly requires a **JS runtime** to solve the `n` parameter challenge. vYtDL defaults to `--js-runtimes node`.

| Symptom | Fix |
|---------|-----|
| `No supported JavaScript runtime` / `n challenge solving failed` | Install Node.js, or `brew install deno` and pass `--js-runtimes deno` |
| `HTTP Error 403` on formats | Update yt-dlp; add `--cookies-from-browser chrome`; optionally `--extractor-args` |
| `Sign in to confirm you're not a bot` | Cookies from a logged-in browser profile |
| `The page needs to be reloaded` | Fresh yt-dlp + node + cookies |
| SSL / proxy errors | Fix proxy env, or `--proxy`, or `--force-ipv4` |

Recommended recovery command:

```bash
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  --js-runtimes node \
  --extractor-args "youtube:player_client=web,android" \
  --force-ipv4 \
  --retries 10 \
  --socket-timeout 30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

Keep yt-dlp current:

```bash
./vYtDL download --install-yt-dlp
# or: python3 -m pip install -U --user --break-system-packages yt-dlp
```

### Concurrent YouTube URLs

```bash
./vYtDL download --no-tui -j 2 -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=ID1" \
  "https://www.youtube.com/watch?v=ID2"
```

Higher `-j` increases bot-check risk; prefer `1–3`.

---

## Bilibili（哔哩哔哩）

### URL shapes

| Kind | Example |
|------|---------|
| Single / multi-P | `https://www.bilibili.com/video/BVxxxxxx` |
| `av` id | `https://www.bilibili.com/video/av123456` |
| Short link | `https://b23.tv/xxxxx` |
| Season / bangumi | `https://www.bilibili.com/bangumi/play/ss…` or `ep…` |
| Favorites / collections | use `--playlist` |

Multi-part（分 P）videos share one BV URL. Use `--playlist` to download all parts into a titled subdirectory with resume.

### Single video (one P)

```bash
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BV1xx411c7mD"
```

Without `--playlist`, only the default / current part is fetched (`--no-playlist`).

### Multi-P / collection / season

```bash
# All parts under one BV
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# Bangumi / season episode list
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/bangumi/play/ss12345"
```

Resume is the same as YouTube playlists: re-run the identical command; state lives in:

```text
./downloads/<标题>/.playlist_state.json
```

### Cookies and VIP / region

| Need | Approach |
|------|----------|
| Higher quality / 大会员 | `--cookies-from-browser chrome` (logged-in bilibili.com session) |
| Netscape file | `--cookies ./cookies.txt` |
| Restricted region | `--proxy "http://127.0.0.1:7890"` |

```bash
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  --proxy "socks5://127.0.0.1:7890" \
  "https://www.bilibili.com/video/BVxxxxxx"
```

### Bilibili vs YouTube behavior notes

| Topic | Notes |
|-------|--------|
| Subtitles | Official / CC tracks vary; auto-subs less common than YouTube. Defaults still request `en,zh`. |
| Titles | Chinese titles are fine; filesystem-safe sanitization applies on playlist dirs. |
| Entry URLs | Playlist entries use `webpage_url` from yt-dlp; vYtDL must not invent `youtube.com/watch` URLs. |
| Quality | `-q 1080` still maps to height filter; actual max depends on login / VIP. |

Equivalent raw yt-dlp:

```bash
yt-dlp -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]" \
  --merge-output-format mp4 \
  --write-subs --write-auto-subs --sub-langs en,zh \
  --cookies-from-browser chrome \
  -o "./downloads/%(title)s.%(ext)s" \
  "https://www.bilibili.com/video/BVxxxxxx"

# vYtDL
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"
```

---

## Build

```bash
# Host binary
go build -trimpath -ldflags "-s -w" -o vYtDL .
# or: task build

# Cross-compile (macOS / Linux / Windows amd64+arm64)
./scripts/build.sh
ONLY=windows ./scripts/build.sh
# task build:windows          (standalone Taskfile)
# task cli:build:windows      (monorepo root Taskfile)

# PowerShell
.\scripts\build.ps1 -Only windows

# Embed yt-dlp into the binary
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
EMBED=1 ONLY=windows ./scripts/build.sh
```

Artifacts: `dist/vYtDL-<os>-<arch>[.exe]`. Download outputs (`*.mp4`, `*.part`, `*.ytdl`, records) are gitignored.

---

## Playlist resume (any site)

Implementation (`internal/playliststate` + `DownloadPlaylist`):

1. Dump playlist entries once.
2. Persist queue + per-item status in `.playlist_state.json`.
3. Skip `succeeded`; retry `failed` / unfinished.
4. `--reset-playlist-state` clears the file.

```bash
./vYtDL download --no-tui --playlist -o ./downloads \
  "https://www.youtube.com/playlist?list=PLxxxx"
# interrupt, then re-run the same command
```

---

## Concurrent downloads

```bash
./vYtDL download --no-tui -j 3 -o ./downloads \
  "URL1" "URL2" "URL3"
```

Worker pool with semaphore (`cmd/download.go`). Records are mutex-protected.

---

## Output files

| File | Purpose |
|------|---------|
| `download_record.json` / `.csv` | success/fail, URL, paths, errors, timing |
| `subtitle_mapping.json` / `.csv` | video ↔ subtitle paths |
| `<playlist>/.playlist_state.json` | resume queue |

Change names/format with `--record-file`, `--mapping-file`, `--log-format csv`.

---

## Subtitle analysis

```bash
./vYtDL analyze --mode text video.en.vtt
./vYtDL analyze --mode text --output transcript.txt video.zh.vtt
./vYtDL analyze < video.vtt
```

Handles YouTube auto-generated VTT (`<c>` tags). Aliases: `an`, `ana`.

---

## Background service (`serve`)

`serve` runs the vYtDL daemon: an HTTP API + WebSocket server that owns the
download queue, SQLite persistence, VTT analysis, and optional static frontend
hosting. It is the single backend for the web UI (see
`docs/refactor/sprint-0/` in the monorepo).

```bash
# Desktop-shell style: random loopback port, serve the exported frontend
./vYtDL serve --port 0 --static ./out

# Docker deployment: all interfaces, fixed port
./vYtDL serve --host 0.0.0.0 --port 3000 --static /app/out --home /app/data

# Local dev: request logging
./vYtDL serve --dev --home ./.daemon-home
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--port` | `0` (`PORT` env) | Listen port; `0` picks a random free port |
| `--host` | `127.0.0.1` (`VYTDL_HOST`) | Bind address; `0.0.0.0` for Docker |
| `--static` | `VYTDL_STATIC_DIR` | Serve the exported frontend from this dir (SPA fallback included) |
| `--home` | `~/.vytdl` (`VYTDL_HOME`) | Data root — `vytdl.db` and logs live here |
| `--dev` | off | Request logging |

Notes:

- Default download dir is `<home>/downloads` (override: `VYTDL_OUTPUT_DIR`).
- The API mirrors the previous web backend one-to-one (`POST /api/start-download`,
  `GET /api/downloads`, `WS /api/ws`, …) — the existing web frontend works unchanged.
- On startup the daemon resumes downloads that were `pending`/`downloading`
  when it last stopped (their serialized options are stored in the `options`
  column). `GET /api/health` reports readiness for the desktop shell.

---

## Other sites

Same CLI; cookies often required:

```bash
./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"

./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://x.com/user/status/123"

./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.xiaohongshu.com/explore/xxxxxx"
```

List extractors: `yt-dlp --list-extractors`. See also `docs/suggestions/supported-platforms.md` in the monorepo.

---

## Shell wrappers

```bash
./scripts/download_video.sh "https://www.youtube.com/watch?v=ID" ./downloads 1080
./scripts/download_collection.sh "https://www.youtube.com/playlist?list=PLxxxx" ./downloads 720
```

---

## Passthrough flags

- `--yt-dlp-bin`
- `--proxy`
- `--cookies` / `--cookies-from-browser`
- `--user-agent`
- `--extractor-args`
- `--js-runtimes` (default `node`)
- `--retries` / `--socket-timeout` / `--force-ipv4`

---

## Troubleshooting

| Problem | Action |
|--------|--------|
| yt-dlp missing | `--install-yt-dlp`, embed build, or set `--yt-dlp-bin` |
| GitHub slow | `VYTDL_YTDLP_MIRROR` |
| YouTube 403 / n-challenge | Node/Deno + cookies + update yt-dlp (see [YouTube](#youtube)) |
| Bilibili low quality | Login cookies / VIP |
| Playlist resumes wrong items | Same `-o` and URL; or `--reset-playlist-state` |
| go.work errors | `GOWORK=off go build …` or use Taskfile (`env.GOWORK=off`) |
