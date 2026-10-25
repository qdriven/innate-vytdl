---
name: vytdl-cli
description: |
  构建、运行和扩展 vYtDL CLI（yt-dlp 封装的 Go 下载器）。当需要用命令行下载视频、
  处理播放列表/分 P、批量下载、VTT 字幕分析、多站点（YouTube/Bilibili/TikTok/X/小红书）
  下载、排查 yt-dlp 报错，或对比 vYtDL 与原生 yt-dlp 的用法时使用。
  触发词：vYtDL、innate-vytdl、视频下载、yt-dlp、playlist resume、download_record、
  --cookies-from-browser、B站下载、YouTube 下载。
---

# vYtDL CLI Skill

vYtDL 是一个独立的 Go CLI/TUI，封装 **yt-dlp** 子进程：简化常用参数、播放列表断点续传、下载记录、VTT 分析。不是独立下载引擎，站点覆盖 = yt-dlp 的 ~1800 个 extractor。

## 仓库

| 位置 | 路径 |
|---|---|
| 本地项目（规范源） | `/Users/patrick/innate/innate-works/projects/innate-vytdl` |
| GitHub | https://github.com/qdriven/innate-vytdl |

Module：`github.com/innate/yt-dl`。项目完全独立，不依赖 innate-keepthem 仓库。

## 何时使用本 skill

- 构建 / 安装 / 调试 vYtDL CLI
- 添加下载参数或改下载行为
- 各站点下载示例（任何 yt-dlp 支持的站点）
- 解释 vYtDL 与原生 `yt-dlp` 的差异
- `analyze` 子命令（VTT → 文本）

桌面版 / Web UI 不在本 skill 范围内（在 innate-keepthem 仓库）。

## 构建与运行

```bash
cd /Users/patrick/innate/innate-works/projects/innate-vytdl
go build -o vYtDL .                     # 仓库内构建无需 GOWORK（项目已独立）
./scripts/build.sh                      # 交叉编译 -> dist/
EMBED=1 ./scripts/build.sh              # 内嵌 yt-dlp 的单文件版

./vYtDL download --help
./vYtDL download --install-yt-dlp       # 把 yt-dlp 缓存到本地（无需系统安装）
./vYtDL download --no-tui -o ./downloads -q 1080 "URL"
```

### yt-dlp 二进制解析顺序

`--yt-dlp-bin` / `config.json` `yt_dlp_bin` / `YT_DL_BIN` → PATH → 内嵌（`-tags embed_ytdlp`）→ 本地缓存（`~/Library/Caches/vYtDL`）→ GitHub 自动下载。

GitHub 慢：`export VYTDL_YTDLP_MIRROR='https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/'`

需要：Go 1.24+（仅构建）；FFmpeg 可选（片段裁剪 / 合并）。

## 架构

```
main.go -> cmd.Execute()
  download  -> internal/downloader（yt-dlp 子进程）+ 可选 TUI
  analyze   -> internal/vtt
```

| 路径 | 职责 |
|------|------|
| `cmd/download.go` | 参数解析、并发、编排 |
| `cmd/analyze.go` | VTT → 文本/markdown |
| `internal/downloader/` | buildArgs、进度解析、播放列表续传、X/Twitter syndication 回退 |
| `internal/ytdlpbin/` | yt-dlp 解析/安装（PATH、内嵌、缓存下载） |
| `internal/tui/` | Bubble Tea 进度 UI |
| `internal/record/` | `download_record` + `subtitle_mapping` |
| `internal/playliststate/` | `.playlist_state.json` 续传状态 |
| `USAGE.md` | 完整使用文档（中文） |

## 常用命令速查

```bash
# YouTube 单视频（默认 EN+ZH 字幕、mp4、1080p）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome "https://www.youtube.com/watch?v=ID"

# Bilibili 多 P / 合集（断点续传：重跑同一命令即可跳过已完成项）
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  "https://www.bilibili.com/video/BVxxx"

# X/Twitter 公开视频 — 无需额外参数（自动走 syndication API）
./vYtDL download --no-tui -o ./downloads "https://x.com/user/status/123"

# 片段裁剪（需 FFmpeg）
./vYtDL download --no-tui -o ./downloads --start 00:01:00 --end 00:02:30 "URL"

# 并发多 URL（建议 1-3）
./vYtDL download --no-tui -j 2 -o ./downloads "URL1" "URL2"

# VTT 字幕分析
./vYtDL analyze --mode text video.en.vtt
```

## 与原生 yt-dlp 的差异

| 意图 | yt-dlp | vYtDL |
|--------|--------|-------|
| 1080p | `-f "bestvideo[height<=1080]+…"` 长串 | `-q 1080` |
| 字幕 | 多个 flag | 默认 EN+ZH 含自动字幕；`--no-subs` 关闭 |
| 输出 | `-o 'dir/%(title)s.%(ext)s'` | `-o ./dir` |
| 裁剪 | `--download-sections` | `--start` / `--end` |
| 播放列表 | 手动 | `--playlist` + 续传 + 标题子目录 |
| 进度 | 文本 | TUI（默认）；`--no-tui` 纯文本 |
| 记录 | 自理 | 自动 `download_record` + 字幕映射 |
| X/Twitter | guest GraphQL 常失败 | 自动 `twitter:api=syndication`（用户显式 `twitter:api=` 优先） |

Cookie / proxy / extractor-args / user-agent / retries 原样透传。

## 扩展 CLI

1. 在 `cmd/download.go` `init()` 加 flag
2. 在 `internal/downloader.Options` 加字段
3. 接入 `downloader.go` 的 `buildArgs()`
4. 用户可见的改动同步 `USAGE.md` 和本 skill

测试：`go test ./...`

## 常见问题

- **yt-dlp 未找到** → `./vYtDL download --install-yt-dlp`，或内嵌构建，或 `--yt-dlp-bin`
- **GitHub 下载慢** → 设 `VYTDL_YTDLP_MIRROR`（见构建一节）
- **Bot / 年龄 / 地区限制** → `--cookies-from-browser chrome` 或 `--cookies cookies.txt`
- **YouTube n-challenge / 403** → 需要 JS 运行时（默认 `--js-runtimes node`，无 node 可 `brew install deno` 后传 `--js-runtimes deno`）；快速修复：`--extractor-args "youtube:player_client=web,android"`（2026-08 验证有效，无需 deno）
- **WorkBuddy 会话代理 502（YouTube 不可达）** → WorkBuddy 注入的 `HTTP(S)_PROXY=127.0.0.1:52711` 可能报 `Tunnel connection failed: 502`；用 `scutil --proxy` 查 macOS 系统代理后覆盖，如 `HTTPS_PROXY=http://127.0.0.1:4780 ./vYtDL download ...`。先验证：`curl -m 10 -x <proxy> https://www.youtube.com`
- **播放列表续传错乱** → 保持相同 `-o` 与 URL 重跑；或 `--reset-playlist-state` 重来

## 相关文档

- `USAGE.md` — 完整使用文档
- `skills/x-video-dl/` — X/Twitter 视频下载 skill（轻量 yt-dlp 直调方案）
- `docs/skills-guide.md` — skill 在 WorkBuddy / zcode / Kimi 中的安装与使用
- `docs/dsh-plugin-model.md` — DSH 插件模式如何落地到本项目所属的 vYtDL 产品线

---
*规范源：本仓库 `skills/vytdl-cli/`。安装位置：`~/.cursor/skills`、`~/.codex/skills`、`~/.trae/skills`、`~/.workbuddy/skills`、`~/.zcode/skills`、`~/.kimi-code/skills`（复制安装，更新以规范源为准）。*
