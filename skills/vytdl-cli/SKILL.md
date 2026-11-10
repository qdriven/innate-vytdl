---
name: vytdl-cli
description: |
  构建、运行和扩展 vYtDL CLI（yt-dlp 封装的 Go 下载器 + serve 守护进程）。当需要用命令行下载视频、
  处理播放列表/分 P、批量下载、查询下载历史、VTT 字幕分析、启动 Web/桌面后端、多站点
  （YouTube/Bilibili/TikTok/X/小红书）下载、排查 yt-dlp 报错，或对比 vYtDL 与原生 yt-dlp
  的用法时使用。
  触发词：vYtDL、innate-vytdl、视频下载、yt-dlp、playlist resume、download_record、
  vYtDL records、--cookies-from-browser、B站下载、YouTube 下载、vYtDL serve。
---

# vYtDL CLI Skill

vYtDL 是一个独立的 Go CLI/TUI，封装 **yt-dlp** 子进程：简化常用参数、播放列表断点续传、下载记录、VTT 分析，并提供 `serve` 后端。不是独立下载引擎，站点覆盖 = yt-dlp 的 ~1800 个 extractor。

## 仓库

| 位置 | 说明 |
|---|---|
| GitHub | https://github.com/qdriven/innate-vytdl |
| 本地 | 本 skill 所在仓库根目录（`skills/` 的上一级） |

Module：`github.com/innate/yt-dl`。二进制名 `vYtDL`；Cobra 根命令 `Use` 是 `yt-dl`（`--help` 里显示 `yt-dl`）。项目完全独立，不依赖其他仓库。

## 何时使用本 skill

- 构建 / 安装 / 调试 vYtDL CLI
- 添加下载参数或改下载行为
- 各站点下载示例（任何 yt-dlp 支持的站点）
- 查询下载历史（`records`）
- `analyze` 子命令（VTT → 文本 / Markdown）
- 启动 / 调试 `serve` 后端
- 解释 vYtDL 与原生 `yt-dlp` 的差异

## 构建与运行

```bash
cd "$(git rev-parse --show-toplevel)"

go build -o vYtDL .                     # 需要 Go 1.26+
./scripts/build.sh                      # 交叉编译 -> dist/
EMBED=1 ./scripts/build.sh              # 内嵌 yt-dlp 的单文件版
go test ./...

./vYtDL --help
./vYtDL download --help
./vYtDL download --install-yt-dlp       # 把 yt-dlp 缓存到本地（无需系统安装）
./vYtDL download --no-tui -o ./downloads -q 1080 "URL"
```

依赖：Go 1.26+（仅构建）；FFmpeg 可选（片段裁剪 / 合并 / serve 音频提取）；Node.js 或 Deno 用于 YouTube n-challenge（默认 `--js-runtimes node`）。

### yt-dlp 二进制解析顺序

`--yt-dlp-bin` / `config.json` `yt_dlp_bin` / `YT_DL_BIN` → PATH（`yt-dlp` / `youtube-dl`）→ 内嵌（`-tags embed_ytdlp`）→ 用户缓存目录下的 `vYtDL`（macOS `~/Library/Caches/vYtDL`，Linux `~/.cache/vYtDL`，Windows `%LocalAppData%\vYtDL`）→ GitHub 自动下载。

GitHub 慢：`export VYTDL_YTDLP_MIRROR='https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/'`

## 架构

```text
main.go -> cmd.Execute()
  download  -> internal/downloader（yt-dlp 子进程）+ 可选 TUI
  records   -> internal/record（统一下载历史）
  analyze   -> internal/vtt
  serve     -> internal/server（队列在 SQLite，历史仍写入同一份目录）
```

| 路径 | 职责 |
|------|------|
| `cmd/download.go` | 参数解析、并发、编排、历史写入 |
| `cmd/records.go` | 查询统一下载历史，并导入旧的双文件记录 |
| `cmd/analyze.go` | VTT → 文本 / markdown |
| `cmd/serve.go` | daemon 启动参数、优雅退出 |
| `internal/downloader/` | buildArgs、进度解析、播放列表续传、X/Twitter syndication 回退 |
| `internal/ytdlpbin/` | yt-dlp 解析/安装（PATH、内嵌、缓存下载） |
| `internal/tui/` | Bubble Tea 进度 UI |
| `internal/record/` | 统一历史 `~/.vytdl/download_record.json`（含字幕路径） |
| `internal/playliststate/` | `.playlist_state.json` 续传状态，不是下载历史 |
| `internal/vtt/` | WebVTT 解析（analyze 与 serve 共用） |
| `internal/server/` | `serve`：HTTP API + WS + 队列 + SQLite + VTT 分析 + 音频提取 |

## 常用命令速查

```bash
# YouTube 单视频（默认 EN+ZH 字幕、mp4、最高画质；-q 1080 限高）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome "https://www.youtube.com/watch?v=ID"

# Bilibili 多 P / 合集（断点续传：重跑同一命令即可跳过已完成项）
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  "https://www.bilibili.com/video/BVxxx"

# X/Twitter 公开视频 — 必须是 status 链接，无需额外参数（自动走 syndication API）
./vYtDL download --no-tui -o ./downloads "https://x.com/user/status/123"

# 片段裁剪（需 FFmpeg）
./vYtDL download --no-tui -o ./downloads --start 00:01:00 --end 00:02:30 "URL"

# 并发多 URL（建议 1-3）
./vYtDL download --no-tui -j 2 -o ./downloads "URL1" "URL2"

# VTT 字幕分析（text / markdown；可读 stdin）
./vYtDL analyze --mode text video.en.vtt
./vYtDL analyze --mode markdown -o transcript.md video.zh.vtt

# 下载记录（是否下过、字幕路径、失败原因）
./vYtDL records --json --limit 20

# 起后端（API + WS；可托管静态前端）
./vYtDL serve --port 0 --static ./out
```

## 默认值与语义

| 项 | 默认 |
|----|------|
| 容器 | `-f mp4`（传给 `--merge-output-format`，不转码） |
| 画质 | 空 = 最高；`-q 720/1080/2160` 是高度上限；其他字符串按 yt-dlp `-f` 原样 |
| 字幕 | `--write-subs --write-auto-subs --sub-langs en,zh` |
| JS 运行时 | `--js-runtimes node` |
| 并发 | `-j 1`；播放列表内部逐条 |
| 历史 | `$VYTDL_HOME/download_record.json`（默认 `~/.vytdl/download_record.json`） |
| `serve` 端口 | `0` = 随机；`PORT` 可给默认 |
| `serve` 队列并发 | 设置 `max_concurrent_downloads`，默认 3（1–10） |

## 与原生 yt-dlp 的差异

| 意图 | yt-dlp | vYtDL |
|--------|--------|-------|
| 1080p | `-f "bestvideo[height<=1080]+…"` 长串 | `-q 1080` |
| 字幕 | 多个 flag | 默认 EN+ZH 含自动字幕；`--no-subs` 关闭 |
| 输出 | `-o 'dir/%(title)s.%(ext)s'` | `-o ./dir` |
| 裁剪 | `--download-sections` | `--start` / `--end` |
| 播放列表 | 手动 | `--playlist` + 续传 + 标题子目录 |
| 进度 | 文本 | TUI（默认）；`--no-tui` 纯文本；无 TTY 自动回退 |
| 记录 | 自理 | 自动写入统一历史（含字幕路径）；`vYtDL records` 查询 |
| 后端 | 无 | `serve`：HTTP API + WS + 队列 + SQLite |
| X/Twitter 单条推文 | guest GraphQL 常失败 | 自动 `twitter:api=syndication`（用户显式 `twitter:api=` 优先）。只认 `/status/<id>`，不能按 handle 下时间线 |

Cookie / proxy / extractor-args / user-agent / retries 原样透传。

## 扩展 CLI

1. 在 `cmd/download.go` `init()` 加 flag
2. 在 `internal/downloader.Options` 加字段
3. 接入 `downloader.go` 的 `buildArgs()`
4. 用户可见的改动同步 `USAGE.md`、`docs/spec.md` 和本 skill。下载历史的字段或路径变了，同时改 `skills/download-record/SKILL.md`。

测试：`go test ./...`

## B站

可以，而且只走本 skill。`x-video-dl` 只下 X/Twitter。

vYtDL 把 B 站 URL 交给本机 yt-dlp（无域名白名单）。2026-11-07 用 yt-dlp 2026.08.19 核对过公开稿件 `BV13x41117TL`：extractor `BiliBili`，容器 mp4。完整地址表（多分 P、番剧、空间投稿、收藏夹、合集、课堂、动态、分区、搜索、国际站、直播）在 `docs/bilibili.md`。

```bash
# 当前这一 P（默认 --no-playlist；指定 P 用 URL ?p=N）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# 全部分 P / 季 / 收藏夹 / 空间投稿
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "URL"
```

高画质和 CC 字幕用已登录 bilibili.com 的 cookie。`b23.tv` 短链没有专用 extractor，先换成跳转后的 `bilibili.com` 地址。大会员专享和未购买的课堂下不了。

## X / Twitter

只下载**单条推文**。地址必须是 `https://x.com/<user>/status/<id>`（或 `twitter.com`、`t.co` 短链）。公开推文自动加 `twitter:api=syndication`。

只给 handle、主页或 `/media` 时，下不了该账号的视频，也筛不了「近 3 个月」。yt-dlp 2026.08.19 没有用户时间线 extractor；syndication 按一条推文取视频；`--playlist` 不会枚举发帖；CLI 没有日期过滤参数。不要把 `https://x.com/<handle>` 配上 `--playlist` 去试。

已有多条 status 时，可以一次传入多个 URL。一条推文里有多段视频时，给这条 status 加 `--playlist`。保护帖、年龄限制要用 cookie，并显式 `--extractor-args "twitter:api=graphql"`（这会关掉 syndication）。细节在 `skills/x-video-dl/SKILL.md`。

## serve

`./vYtDL serve [--port N] [--host ADDR] [--static DIR] [--home DIR] [--dev]`

- 默认 `127.0.0.1`、随机端口（`--port 0`）；Docker 用 `--host 0.0.0.0`。
- 数据根默认 `~/.vytdl`：`vytdl.db`（实时队列 / 设置 / VTT 报告）+ `download_record.json`（历史）。
- 默认下载目录 `<home>/downloads`，`VYTDL_OUTPUT_DIR` 可覆盖。
- 启动时恢复 `pending` / `downloading`；取消 / 重试 / 删除可用；暂停 / 恢复返回 `success:false`。
- `GET /api/health` 用于就绪探测；`WS /api/ws` 推送进度与队列事件。
- 每个下载结束后追加历史，与 CLI `records` 同一份。

完整端点、请求字段、设置键见 `USAGE.md#后台服务serve`。

## 常见问题

- **yt-dlp 未找到** → `./vYtDL download --install-yt-dlp`，或内嵌构建，或 `--yt-dlp-bin`
- **GitHub 下载慢** → 设 `VYTDL_YTDLP_MIRROR`
- **Bot / 年龄 / 地区限制** → `--cookies-from-browser chrome` 或 `--cookies cookies.txt`
- **YouTube n-challenge / 403** → 需要 JS 运行时（默认 `--js-runtimes node`，无 node 可 `brew install deno` 后传 `--js-runtimes deno`）；快速修复：`--extractor-args "youtube:player_client=web,android"`（2026-08 验证有效，无需 deno）
- **WorkBuddy 会话代理 502（YouTube 不可达）** → WorkBuddy 注入的 `HTTP(S)_PROXY=127.0.0.1:52711` 可能报 `Tunnel connection failed: 502`；用 `scutil --proxy` 查 macOS 系统代理后覆盖，如 `HTTPS_PROXY=http://127.0.0.1:4780 ./vYtDL download ...`。先验证：`curl -m 10 -x <proxy> https://www.youtube.com`
- **播放列表续传错乱** → 保持相同 `-o` 与 URL 重跑；或 `--reset-playlist-state` 重来
- **`records` 看不到刚下的** → 历史在整条命令结束后写入；确认 `VYTDL_HOME` / `--record-file` 一致

## 相关文档

- `docs/spec.md` — CLI 用途与能力边界
- `docs/use-cases.md` — YouTube / B 站 / X / serve / Skill 端到端用例
- `docs/bilibili.md` — 当前 yt-dlp 能下载的 B 站内容
- `USAGE.md` — 完整使用文档（参数、站点、serve API、排障）
- `skills/download-record/` — 查询统一下载历史
- `skills/x-video-dl/` — X/Twitter 视频下载 skill（轻量 yt-dlp 直调方案）
- `docs/skills-guide.md` — skill 在 WorkBuddy / zcode / Kimi 中的安装与使用

---

*规范源：本仓库 `skills/vytdl-cli/`。安装位置：`~/.cursor/skills`、`~/.codex/skills`、`~/.trae/skills`、`~/.workbuddy/skills`、`~/.zcode/skills`、`~/.kimi-code/skills`（复制安装，更新以规范源为准）。*
