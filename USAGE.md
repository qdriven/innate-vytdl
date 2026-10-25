# vYtDL 使用文档

Go CLI / TUI，封装 [yt-dlp](https://github.com/yt-dlp/yt-dlp)。vYtDL **不重新实现下载器** —— 它调度 yt-dlp 子进程，简化日常参数，并补充播放列表断点续传、TUI 进度和下载记录。

**YouTube 与 Bilibili 的完整指南**见下文；其他站点（TikTok、X、小红书…）用法相同，见[多站点](#多站点)一节。

---

## 快速开始

```bash
cd innate-vytdl
go build -o vYtDL .

./vYtDL download --install-yt-dlp   # 可选：缓存一份新的 yt-dlp

# YouTube 单视频（1080p，默认 EN+ZH 字幕）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Bilibili 多 P / 合集
./vYtDL download --no-tui -o ./downloads -q 1080 --playlist \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"
```

值得了解的默认值：

| 默认 | 值 |
|---------|--------|
| 容器 | `-f mp4` |
| 字幕 | EN + ZH（含自动字幕） |
| JS 运行时（YouTube） | `--js-runtimes node` |
| 并发 | `-j 1`（串行） |
| 进度 | TUI（`--no-tui` 为纯文本） |
| 记录 | `download_record.json` + `subtitle_mapping.json` |

---

## 工作原理（实现）

```
./vYtDL download [flags] URL…
        │
        ▼
  cmd/download.go          解析 flags → downloader.Options
        │
        ▼
  internal/ytdlpbin        解析 yt-dlp 二进制
        │
        ▼
  internal/downloader      buildArgs() → exec yt-dlp
        │                    解析 --newline 进度
        │                    可选 playliststate 续传
        ▼
  internal/record          追加 download_record + subtitle_mapping
  internal/tui (可选)       Bubble Tea 进度
```

### 参数 → yt-dlp 映射

| vYtDL 参数 | yt-dlp 等价（近似） |
|-----------|------------------------------|
| `-q 1080` | `-f "bestvideo[height<=1080]+bestaudio/best[height<=1080]"` |
| `-f mp4` | `--merge-output-format mp4` |
| `-o ./dir` | `-o "./dir/%(title)s.%(ext)s"` |
| （默认字幕） | `--write-subs --write-auto-subs --sub-langs en,zh` |
| `--start` / `--end` | `--download-sections "*START-END" --force-keyframes-at-cuts` |
| `--no-playlist`（默认） | `--no-playlist` |
| `--playlist` | 播放列表模式 + 标题子目录 + `.playlist_state.json` |
| `--cookies-from-browser` | 相同 |
| `--cookies` | 相同 |
| `--proxy` | 相同 |
| `--extractor-args` | 相同（X/Twitter 链接自动追加 `twitter:api=syndication`，除非用户设置了 `twitter:api`） |
| `--js-runtimes node` | 相同（vYtDL 默认） |
| `--retries` / `--socket-timeout` / `--force-ipv4` | 相同 |

代码路径：`cmd/download.go` → `internal/downloader/options.go` → `buildArgs()`（`internal/downloader/downloader.go`）。

### yt-dlp 二进制解析

顺序（`internal/ytdlpbin`）：

1. `--yt-dlp-bin` / `config.json` `yt_dlp_bin` / `YT_DL_BIN`
2. PATH 上的 `yt-dlp` / `youtube-dl`
3. 内嵌二进制（`-tags embed_ytdlp`）
4. 缓存：`~/Library/Caches/vYtDL`（macOS）或 `~/.cache/vYtDL`
5. GitHub Releases 自动下载

```bash
./vYtDL download --install-yt-dlp
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

`config.json` 中 `yt_dlp_bin` 为空表示"自动解析"。仅当需要固定路径时才复制 `config.example.json`：

```json
{ "yt_dlp_bin": "/opt/homebrew/bin/yt-dlp" }
```

---

## YouTube

### URL 形态

| 类型 | 示例 |
|------|---------|
| Watch | `https://www.youtube.com/watch?v=VIDEO_ID` |
| 短链 | `https://youtu.be/VIDEO_ID` |
| Shorts | `https://www.youtube.com/shorts/VIDEO_ID` |
| 播放列表 | `https://www.youtube.com/playlist?list=PLxxxx` |
| Mix / 电台 | `list=RD…` — 单视频 Watch 链接保持默认 `--no-playlist` |

转义过的 URL（如 `watch\?v\=ID`）会自动归一化。

### 单视频

```bash
# 1080p 以下最高画质，EN+ZH 字幕，纯文本进度
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=N2GFDDZtGQY"

# 片段裁剪（需 FFmpeg）
./vYtDL download --no-tui -o ./downloads \
  --start 00:01:00 --end 00:02:30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# 不要字幕
./vYtDL download --no-tui -o ./downloads --no-subs \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# 只要中文字幕
./vYtDL download --no-tui -o ./downloads --sub-langs zh \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

等价的裸 yt-dlp 命令（vYtDL 近似还原的目标）：

```bash
yt-dlp -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]" \
  --merge-output-format mp4 \
  --write-subs --write-auto-subs --sub-langs en,zh \
  --js-runtimes node \
  --cookies-from-browser chrome \
  -o "./downloads/%(title)s.%(ext)s" \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

### 播放列表

```bash
./vYtDL download --no-tui --playlist -o ./downloads -q 720 \
  --cookies-from-browser chrome \
  "https://www.youtube.com/playlist?list=PLxxxx"
```

流程：

1. 拉取播放列表元数据。
2. 创建 `./downloads/<播放列表标题>/`。
3. 写入 `.playlist_state.json`，逐条下载。
4. **重跑同一命令即可续传**；已成功条目自动跳过。
5. `--reset-playlist-state` 丢弃状态、从头开始。

```text
./downloads/My Playlist/
  .playlist_state.json
  Video One.mp4
  Video One.en.vtt
  …
./downloads/download_record.json
./downloads/subtitle_mapping.json
```

### YouTube 反爬 / 403 / “n challenge”

YouTube 越来越多地要求 **JS 运行时**来解 `n` 参数挑战。vYtDL 默认传 `--js-runtimes node`。

| 症状 | 处理 |
|---------|-----|
| `No supported JavaScript runtime` / `n challenge solving failed` | 装 Node.js，或 `brew install deno` 后传 `--js-runtimes deno` |
| 格式列表 `HTTP Error 403` | 更新 yt-dlp；加 `--cookies-from-browser chrome`；必要时 `--extractor-args` |
| `Sign in to confirm you're not a bot` | 用已登录浏览器 profile 的 cookie |
| `The page needs to be reloaded` | 更新 yt-dlp + node + cookie |
| SSL / 代理错误 | 修代理环境变量，或 `--proxy`，或 `--force-ipv4` |

推荐恢复命令：

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

保持 yt-dlp 新：

```bash
./vYtDL download --install-yt-dlp
# 或: python3 -m pip install -U --user --break-system-packages yt-dlp
```

### 多 URL 并发

```bash
./vYtDL download --no-tui -j 2 -o ./downloads \
  --cookies-from-browser chrome \
  "https://www.youtube.com/watch?v=ID1" \
  "https://www.youtube.com/watch?v=ID2"
```

`-j` 越高越容易触发 bot 检查；建议 `1–3`。

---

## Bilibili（哔哩哔哩）

### URL 形态

| 类型 | 示例 |
|------|---------|
| 单视频 / 多 P | `https://www.bilibili.com/video/BVxxxxxx` |
| `av` 号 | `https://www.bilibili.com/video/av123456` |
| 短链 | `https://b23.tv/xxxxx` |
| 番剧 / 季 | `https://www.bilibili.com/bangumi/play/ss…` 或 `ep…` |
| 收藏夹 / 合集 | 用 `--playlist` |

多 P（分 P）视频共用一个 BV 链接。用 `--playlist` 把所有分 P 下到一个标题子目录并支持续传。

### 单视频（单 P）

```bash
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BV1xx411c7mD"
```

不带 `--playlist` 时只下载默认 / 当前分 P（`--no-playlist`）。

### 多 P / 合集 / 番剧

```bash
# 一个 BV 下的全部分 P
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# 番剧 / 剧集列表
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/bangumi/play/ss12345"
```

续传方式与 YouTube 播放列表相同：重跑同一命令；状态在：

```text
./downloads/<标题>/.playlist_state.json
```

### Cookie 与大会员 / 地区

| 需求 | 方案 |
|------|----------|
| 更高画质 / 大会员 | `--cookies-from-browser chrome`（已登录 bilibili.com 的会话） |
| Netscape cookie 文件 | `--cookies ./cookies.txt` |
| 地区限制 | `--proxy "http://127.0.0.1:7890"` |

```bash
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  --proxy "socks5://127.0.0.1:7890" \
  "https://www.bilibili.com/video/BVxxxxxx"
```

### Bilibili 与 YouTube 的行为差异

| 主题 | 说明 |
|-------|--------|
| 字幕 | 官方 / CC 轨道不定；自动字幕比 YouTube 少。默认仍请求 `en,zh`。 |
| 标题 | 中文标题没问题；播放列表目录会做文件系统安全清洗。 |
| 条目 URL | 播放列表条目使用 yt-dlp 的 `webpage_url`；vYtDL 不会伪造 `youtube.com/watch` URL。 |
| 画质 | `-q 1080` 仍是高度过滤；实际上限取决于登录 / 大会员。 |

等价裸 yt-dlp：

```bash
yt-dlp -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]" \
  --merge-output-format mp4 \
  --write-subs --write-auto-subs --sub-langs en,zh \
  --cookies-from-browser chrome \
  -o "./downloads/%(title)s.%(ext)s" \
  "https://www.bilibili.com/video/BVxxxxxx"
```

---

## X / Twitter

公开推文**无需任何额外参数**：vYtDL 自动追加 `twitter:api=syndication`（guest GraphQL API 对 api.x.com 经常报 TLS EOF / Cloudflare 拦截，syndication embed API 仍可拿到公开视频）。

```bash
# X / Twitter 公开视频 — 开箱即用
./vYtDL download --no-tui -o ./downloads \
  "https://x.com/user/status/123"

# 私密 / NSFW 推文仍需 cookie；显式 extractor-args 会关闭 syndication 回退
./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  --extractor-args "twitter:api=graphql" \
  "https://x.com/user/status/123"
```

识别的域名：`x.com`、`twitter.com`（含子域）、`vxtwitter.com`、`fxtwitter.com`、`nitter.net`。用户显式传入任何 `twitter:api=` 时自动回退失效（用户优先）。

---

## 多站点

同一个 CLI；TikTok / 小红书 / Instagram 通常需要 cookie。

```bash
./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"

./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.xiaohongshu.com/explore/xxxxxx"
```

列出全部 extractor：`yt-dlp --list-extractors`。

---

## 构建

```bash
# 本机二进制
go build -trimpath -ldflags "-s -w" -o vYtDL .
# 或: task build

# 交叉编译（macOS / Linux / Windows amd64+arm64）
./scripts/build.sh
ONLY=windows ./scripts/build.sh
# task build:windows
# PowerShell: .\scripts\build.ps1 -Only windows

# 内嵌 yt-dlp
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
EMBED=1 ONLY=windows ./scripts/build.sh
```

产物：`dist/vYtDL-<os>-<arch>[.exe]`。下载产物（`*.mp4`、`*.part`、`*.ytdl`、记录文件）已被 gitignore。

---

## 播放列表续传（任意站点）

实现（`internal/playliststate` + `DownloadPlaylist`）：

1. 一次性 dump 播放列表条目。
2. 队列 + 每条状态持久化到 `.playlist_state.json`。
3. 跳过 `succeeded`；重试 `failed` / 未完成。
4. `--reset-playlist-state` 清除状态文件。

```bash
./vYtDL download --no-tui --playlist -o ./downloads \
  "https://www.youtube.com/playlist?list=PLxxxx"
# 中断后重跑同一命令即续传
```

---

## 并发下载

```bash
./vYtDL download --no-tui -j 3 -o ./downloads \
  "URL1" "URL2" "URL3"
```

信号量 worker pool（`cmd/download.go`）。记录文件有互斥保护。

---

## 输出文件

| 文件 | 用途 |
|------|---------|
| `download_record.json` / `.csv` | 成功/失败、URL、路径、错误、耗时 |
| `subtitle_mapping.json` / `.csv` | 视频 ↔ 字幕路径 |
| `<playlist>/.playlist_state.json` | 续传队列 |

用 `--record-file`、`--mapping-file`、`--log-format csv` 改名 / 换格式。

---

## 字幕分析

```bash
./vYtDL analyze --mode text video.en.vtt
./vYtDL analyze --mode text --output transcript.txt video.zh.vtt
./vYtDL analyze < video.vtt
```

支持 YouTube 自动生成 VTT（`<c>` 标签）。别名：`an`、`ana`。

---

## 后台服务（serve）

`serve` 启动 vYtDL daemon：HTTP API + WebSocket 服务，负责下载队列、SQLite 持久化、VTT 分析，可选静态托管前端导出。它是 Web / 桌面前端的唯一后端。

```bash
# 桌面壳模式：随机回环端口，托管前端导出目录
./vYtDL serve --port 0 --static ./out

# Docker 部署：全接口 + 固定端口
./vYtDL serve --host 0.0.0.0 --port 3000 --static /app/out --home /app/data

# 本地开发：请求日志
./vYtDL serve --dev --home ./.daemon-home
```

| 参数 | 默认 | 含义 |
|------|---------|---------|
| `--port` | `0`（`PORT` 环境变量） | 监听端口；`0` = 随机空闲端口 |
| `--host` | `127.0.0.1`（`VYTDL_HOST`） | 绑定地址；Docker 用 `0.0.0.0` |
| `--static` | `VYTDL_STATIC_DIR` | 从该目录托管前端导出（含 SPA 回退） |
| `--home` | `~/.vytdl`（`VYTDL_HOME`） | 数据根目录 — `vytdl.db` 与日志在此 |
| `--dev` | 关 | 请求日志 |

说明：

- 默认下载目录为 `<home>/downloads`（可用 `VYTDL_OUTPUT_DIR` 覆盖）。
- API 与 Web 后端一一对应（`POST /api/start-download`、`GET /api/downloads`、`WS /api/ws` 等）—— 现有 Web 前端无需改动。
- daemon 启动时会恢复上次停止时处于 `pending` / `downloading` 状态的下载（序列化选项存在 `options` 列）。`GET /api/health` 供桌面壳做就绪探测。

---

## Shell 封装

```bash
./scripts/download_video.sh "https://www.youtube.com/watch?v=ID" ./downloads 1080
./scripts/download_collection.sh "https://www.youtube.com/playlist?list=PLxxxx" ./downloads 720
```

---

## 透传参数

- `--yt-dlp-bin`
- `--proxy`
- `--cookies` / `--cookies-from-browser`
- `--user-agent`
- `--extractor-args`
- `--js-runtimes`（默认 `node`）
- `--retries` / `--socket-timeout` / `--force-ipv4`

---

## 排障速查

| 问题 | 处理 |
|--------|--------|
| yt-dlp 缺失 | `--install-yt-dlp`、内嵌构建，或 `--yt-dlp-bin` |
| GitHub 慢 | `VYTDL_YTDLP_MIRROR` |
| YouTube 403 / n-challenge | Node/Deno + cookie + 更新 yt-dlp（见 [YouTube](#youtube)） |
| Bilibili 画质低 | 登录 cookie / 大会员 |
| 播放列表续传错乱 | 相同 `-o` 与 URL 重跑；或 `--reset-playlist-state` |
| X/Twitter 拿不到公开视频 | 默认自动 syndication；仍失败则更新 yt-dlp，私密内容加 cookie |
