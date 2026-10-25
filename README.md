# vYtDL（innate-vytdl）

Go CLI / TUI，封装 [yt-dlp](https://github.com/yt-dlp/yt-dlp)，支持 **1800+ 站点**（YouTube、Bilibili、TikTok、X/Twitter、Instagram、Vimeo、Twitch、小红书…）的视频下载；并自带 **`vYtDL serve` 后台服务**（HTTP API + WebSocket + 下载队列 + SQLite），是 Web / 桌面前端的唯一后端。

vYtDL **不重新实现下载器**：它把 yt-dlp 作为子进程调度，简化日常参数（`-q 1080`、默认 EN/ZH 字幕、`-o` 目录），并补充播放列表断点续传、TUI 进度、下载记录，以及按需自动安装 yt-dlp。

本项目完全独立，不依赖其他仓库。基于 DSH 插件模式的桌面版 / Web UI（宿主 + 插件架构）见 innate-keepthem 仓库的 vYtDL 产品线，架构渊源见 [`docs/dsh-plugin-model.md`](./docs/dsh-plugin-model.md)。

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

完整用法（多站点示例、播放列表续传、serve 参数、排障、与原生 yt-dlp 对照）见 **[USAGE.md](./USAGE.md)**。

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

## Skills（Agent Skill）

本项目自带两个可安装到 AI Agent 工具（WorkBuddy / zcode / Kimi Code / Cursor / Codex / Trae…）的 Skill：

| Skill | 用途 |
|-------|------|
| [`skills/vytdl-cli`](./skills/vytdl-cli/) | 构建 / 运行 / 扩展本 CLI；多站点下载排障 |
| [`skills/x-video-dl`](./skills/x-video-dl/) | X/Twitter 推文视频下载（yt-dlp + 浏览器 cookie 直调） |

安装方法与三个主力工具（WorkBuddy / zcode / Kimi）的任务化用法见 **[docs/skills-guide.md](./docs/skills-guide.md)**。

## 目录结构

```
cmd/                 # Cobra 命令（download、analyze、serve）
internal/downloader/ # yt-dlp 调度 + 进度解析 + 播放列表续传
internal/ytdlpbin/   # yt-dlp 二进制解析（PATH / 内嵌 / 缓存 / 自动下载）
internal/tui/        # Bubble Tea 进度 UI
internal/record/     # download_record / subtitle_mapping
internal/vtt/        # VTT 解析（analyze 子命令）
internal/server/     # serve daemon：HTTP API + WS + 队列 + SQLite
scripts/             # 构建 + fetch-ytdlp + shell 封装
skills/              # Agent Skill（vytdl-cli、x-video-dl）
docs/                # skills 指南、DSH 插件模式说明
USAGE.md             # CLI 完整使用文档
```

## 文档索引

| 文档 | 内容 |
|------|------|
| [USAGE.md](./USAGE.md) | CLI 完整使用指南（中文） |
| [docs/skills-guide.md](./docs/skills-guide.md) | Skill 如何配合 CLI 在 WorkBuddy / zcode / Kimi 中使用 |
| [docs/dsh-plugin-model.md](./docs/dsh-plugin-model.md) | DSH 插件模式如何落地完成第一个项目（vYtDL 桌面版） |
| [skills/README.md](./skills/README.md) | Skill 总览与安装 |

## License

发布时补充 LICENSE 文件。
