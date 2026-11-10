# vYtDL（innate-vytdl）

Go CLI / TUI，封装 [yt-dlp](https://github.com/yt-dlp/yt-dlp)，支持 **1800+ 站点**（YouTube、Bilibili、TikTok、X/Twitter、Instagram、Vimeo、Twitch、小红书…）的视频下载；并自带 **`vYtDL serve` 后台服务**（HTTP API + WebSocket + 下载队列 + SQLite），是 Web / 桌面前端的后端。

vYtDL **不重新实现下载器**：它把 yt-dlp 作为子进程调度，简化日常参数（`-q 1080`、默认 EN/ZH 字幕、`-o` 目录），并补充播放列表断点续传、TUI 进度、统一下载记录，以及按需自动安装 yt-dlp。

本项目是一个独立的 Go module（`github.com/innate/yt-dl`），不依赖其他仓库。产品定位与能力边界见 [`docs/spec.md`](./docs/spec.md)。

## 快速开始

```bash
git clone https://github.com/qdriven/innate-vytdl.git
cd innate-vytdl
go build -o vYtDL .          # 需要 Go 1.26+

./vYtDL download --install-yt-dlp   # 可选：缓存一份 yt-dlp（无需系统安装）

# YouTube 单视频（1080p，默认 EN+ZH 字幕）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Bilibili 多 P / 合集（断点续传）
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  "https://www.bilibili.com/video/BVxxxxxx"

# X/Twitter 公开视频 — 无需额外参数（自动走 syndication API）
./vYtDL download --no-tui -o ./downloads "https://x.com/user/status/123"

# 后台服务（Web UI 后端）：随机端口 + 静态托管前端导出
./vYtDL serve --port 0 --static ./out
```

GitHub 下载慢时：

```bash
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

更多场景（片段裁剪、播放列表续传、多站点批量、字幕分析、Web 后端、Agent Skill）见 **[docs/use-cases.md](./docs/use-cases.md)**；完整参数见 **[USAGE.md](./USAGE.md)**。

## 命令

| 命令 | 作用 |
|------|------|
| `download <url…>`（别名 `dl`、`get`） | 下载视频 / 播放列表；`--install-yt-dlp` 只安装 yt-dlp |
| `records` | 查询统一下载历史（表格 / JSON / 只看失败） |
| `analyze <file.vtt>`（别名 `an`、`ana`） | VTT 字幕 → 纯文本 / Markdown（可读 stdin） |
| `serve` | 启动 HTTP API + WebSocket 守护进程 |
| `completion <shell>` | 生成 shell 补全脚本 |

常用默认值：

| 默认 | 值 |
|------|-----|
| 容器 | `-f mp4`（`--merge-output-format`） |
| 画质 | 空 = 最高 |
| 字幕 | EN + ZH（含自动字幕） |
| JS 运行时（YouTube） | `--js-runtimes node` |
| 并发 | `-j 1`（串行） |
| 进度 | TUI；`--no-tui` 为纯文本，无 TTY 时自动回退 |
| 下载历史 | `~/.vytdl/download_record.json`（每条的 `subtitles` 字段） |

## 构建

```bash
go build -o vYtDL .

# 交叉编译 macOS / Linux / Windows（amd64+arm64）→ dist/
./scripts/build.sh
ONLY=windows ./scripts/build.sh          # 或 task build:windows
# PowerShell: .\scripts\build.ps1 -Only windows

# 内嵌 yt-dlp 的单文件版
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
```

Go 1.26+。FFmpeg 可选：片段裁剪、合并、`serve` 的音频提取需要它。交叉编译脚本在 `scripts/`，`Taskfile.yml` 提供 `task build` / `task test` 等快捷方式。

## yt-dlp 二进制解析

顺序（`internal/ytdlpbin`）：

1. `--yt-dlp-bin` / `config.json` 的 `yt_dlp_bin` / `YT_DL_BIN`
2. PATH 上的 `yt-dlp` / `youtube-dl`
3. 构建时内嵌（`-tags embed_ytdlp`）
4. 用户缓存目录下的 `vYtDL`（macOS `~/Library/Caches/vYtDL`，Linux `~/.cache/vYtDL`，Windows `%LocalAppData%\vYtDL`）
5. 从 GitHub Releases 自动下载

`config.json` 只支持 `yt_dlp_bin`；`VYTDL_CONFIG` 可指定配置文件路径。

## 下载记录

CLI 和 `serve` 把每次下载写进同一个文件，字幕路径在每条记录里：

```bash
./vYtDL records            # 表格，最新在前
./vYtDL records --json     # 含 filename 与 subtitles
./vYtDL records --failed   # 只看失败
```

默认路径是 `~/.vytdl/download_record.json`（可用 `VYTDL_HOME` 或 `--record-file` 改；`--log-format csv` 换成 CSV）。`serve` 的 `vytdl.db` 只保存实时队列；播放列表续传仍是输出目录里的 `.playlist_state.json`。

## 目录结构

```
cmd/                 # Cobra 命令（download、records、analyze、serve、root）
internal/downloader/ # yt-dlp 调度 + 进度解析 + 播放列表续传
internal/ytdlpbin/   # yt-dlp 二进制解析（PATH / 内嵌 / 缓存 / 自动下载）
internal/tui/        # Bubble Tea 进度 UI
internal/record/     # 统一下载历史 ~/.vytdl/download_record.json
internal/vtt/        # VTT 解析（analyze 子命令 + serve 的 VTT 分析）
internal/playliststate/ # .playlist_state.json 续传状态
internal/server/     # serve daemon：HTTP API + WS + 队列 + SQLite
scripts/             # 构建 + fetch-ytdlp + shell 封装
skills/              # Agent Skill（vytdl-cli、download-record、x-video-dl）
docs/                # spec、use-cases、B 站范围、skills 指南
USAGE.md             # CLI 完整使用文档
```

## Skills（Agent Skill）

本项目自带三个可安装到 AI Agent 工具（WorkBuddy / zcode / Kimi Code / Cursor / Codex / Trae…）的 Skill：

| Skill | 用途 | 典型触发 |
|-------|------|----------|
| [`skills/vytdl-cli`](./skills/vytdl-cli/) | 构建 / 运行 / 扩展本 CLI；多站点下载；排障 | 「下载这个 YouTube/B站 视频」「播放列表续传」「vYtDL 报 403」 |
| [`skills/download-record`](./skills/download-record/) | 查询统一下载记录（是否下过、字幕路径、失败原因） | 「这个视频下过没有」「字幕文件在哪」 |
| [`skills/x-video-dl`](./skills/x-video-dl/) | X/Twitter 推文视频下载（yt-dlp + 浏览器 cookie 直调脚本） | 「把这个推文视频存下来」+ x.com 链接 |

安装方法与三个主力工具（WorkBuddy / zcode / Kimi）的任务化用法见 **[docs/skills-guide.md](./docs/skills-guide.md)**；Skill 总览见 [`skills/README.md`](./skills/README.md)。

## 文档索引

| 文档 | 内容 |
|------|------|
| [USAGE.md](./USAGE.md) | CLI 完整使用指南：命令面、参数映射、serve API、排障 |
| [docs/spec.md](./docs/spec.md) | CLI 的用途与能力边界规范（做什么 / 不做什么 / 数据契约） |
| [docs/use-cases.md](./docs/use-cases.md) | 端到端用例：YouTube、B 站、X、多站点、serve、Agent Skill |
| [docs/bilibili.md](./docs/bilibili.md) | 当前 yt-dlp 能下载的 B 站内容，以及登录 / 画质限制 |
| [docs/skills-guide.md](./docs/skills-guide.md) | Skill 如何配合 CLI 在 WorkBuddy / zcode / Kimi 中使用 |
| [skills/README.md](./skills/README.md) | Skill 总览与安装 |
