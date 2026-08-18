---
name: vytdl-cli
description: |
  Build, run, and extend the vYtDL Go CLI (yt-dlp wrapper). Use when downloading
  videos via CLI, adding download flags, playlist resume, TUI, VTT analyze,
  multi-site yt-dlp usage (YouTube/Bilibili/TikTok/X/小红书), or comparing to
  raw yt-dlp. Triggers: vYtDL, yt-dl CLI, go build vYtDL, --cookies-from-browser,
  playlist resume, download_record.
---

# vYtDL CLI Skill

Standalone Go CLI that wraps **yt-dlp** as a subprocess. Not a separate download engine.

## Repo

| 位置 | 路径 |
|---|---|
| GitHub（规范仓库） | https://github.com/qdriven/innate-vytdl |
| 本地工作副本 | `/Users/patrick/innate/innative-works/projects/innate-keepthem/vYtDL-standalone` |
| monorepo 镜像 | `/Users/patrick/innate/innative-works/projects/innate-keepthem/tools/vytdl-cli` |

Module: `github.com/innate/yt-dl`。两处本地副本需保持 Go 源码一致；功能改动优先在 GitHub 仓库开发。

## When to use this skill

- Build / install / debug the CLI
- Add flags or change download behavior
- Multi-platform download examples (any yt-dlp site)
- Explain differences vs raw `yt-dlp`
- VTT subtitle analyze subcommand

Do **not** use this skill for Tauri Desktop / Web UI (those stay in `innate-keepthem`, skill `vytdl-dev`).

## Build & run

```bash
cd /Users/patrick/innate/innative-works/projects/innate-keepthem/vYtDL-standalone
go build -o vYtDL .
./scripts/build.sh                        # cross-compile -> dist/
EMBED=1 ./scripts/build.sh                # cross-compile with yt-dlp embedded

./vYtDL download --help
./vYtDL download --install-yt-dlp         # fetch yt-dlp into cache
./vYtDL download --no-tui -o ./downloads -q 1080 "URL"
```

### yt-dlp provisioning

Order: `--yt-dlp-bin` / config / `YT_DL_BIN` -> PATH -> **embed** (`-tags embed_ytdlp`) -> cache -> GitHub download.

```bash
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
```

Slow GitHub: `export VYTDL_YTDLP_MIRROR='https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/'`

Config: `config.json` / `VYTDL_CONFIG` / `--yt-dlp-bin`. Empty `yt_dlp_bin` -> auto resolve.

Requires: Go 1.24+, optional system yt-dlp (otherwise auto-download/embed), optional FFmpeg.

## Architecture

```
main.go -> cmd.Execute()
  download  -> internal/downloader (yt-dlp subprocess) + optional TUI
  analyze   -> internal/vtt
```

| Path | Role |
|------|------|
| `cmd/download.go` | Flags, concurrency, orchestration |
| `cmd/analyze.go` | VTT -> text/markdown |
| `internal/downloader/` | Args build, progress parse, playlist resume |
| `internal/ytdlpbin/` | Resolve/install yt-dlp (PATH, embed, cache download) |
| `internal/tui/` | Bubble Tea progress UI |
| `internal/record/` | `download_record` + `subtitle_mapping` |
| `internal/playliststate/` | `.playlist_state.json` resume |
| `USAGE.md` | User docs + multi-site examples + vs yt-dlp |

## Platform support

**No domain whitelist.** Pass any URL yt-dlp accepts (~1800 extractors).

```bash
# YouTube
./vYtDL download --no-tui -o ./dl -q 1080 "https://www.youtube.com/watch?v=ID"

# Bilibili playlist / multi-P
./vYtDL download --no-tui -o ./dl --playlist "https://www.bilibili.com/video/BVxxx"

# TikTok / X / 小红书 / Instagram - usually need cookies
./vYtDL download --no-tui -o ./dl --cookies-from-browser chrome "URL"
```

## Simplifications vs raw yt-dlp

| Intent | yt-dlp | vYtDL |
|--------|--------|-------|
| 1080p | long `-f "bestvideo[height<=1080]+…"` | `-q 1080` |
| Subs | several flags | default EN+ZH; `--no-subs` off |
| Output | `-o 'dir/%(title)s.%(ext)s'` | `-o ./dir` |
| Clip | `--download-sections` | `--start` / `--end` |
| Playlist | manual | `--playlist` + resume + titled subdir |
| Progress | text | TUI (default); `--no-tui` plain |
| Logs | DIY | auto `download_record` + subtitle map |

Cookie / proxy / extractor-args pass through unchanged.

## Extending the CLI

1. Add flag in `cmd/download.go` `init()`
2. Add field on `internal/downloader.Options`
3. Wire into `buildArgs()` in `downloader.go`
4. Update `USAGE.md` + this skill if user-facing
5. 双向同步：改动回传另一份本地副本（standalone <-> monorepo tools/vytdl-cli）

Tests: `go test ./internal/downloader/ ./internal/vtt/ …`

## Common issues

- **yt-dlp not found** -> `./vYtDL download --install-yt-dlp`, or embed build, or set `--yt-dlp-bin`
- **GitHub download slow** -> set `VYTDL_YTDLP_MIRROR` (see Build section)
- **Bot / age / region** -> `--cookies-from-browser chrome` or `--cookies cookies.txt`
- **YouTube n-challenge** -> need JS runtime (deno/node); may need `--extractor-args`
- **Playlist empty URLs for non-YouTube** -> entries need `webpage_url`; do not invent YouTube watch URLs

## Related docs

- `USAGE.md` - full usage
- `README.md` - overview
- Monorepo Desktop: `innate-keepthem` + skill `vytdl-dev` (suite, not CLI-only)
- 功能清单与验证：`innate-keepthem/docs/FEATURE-CHECKLIST.md` 第 1 节

---
*本 skill 由 innate-keepthem 仓库维护并分发；规范源：`vYtDL-standalone/.agents/skills/vytdl-cli/`。安装位置：~/.cursor/skills、~/.codex/skills、~/.trae/skills、~/.workbuddy/skills（复制安装，更新以规范源为准）。*
