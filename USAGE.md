# vYtDL 使用文档

Go CLI / TUI + 后台守护进程，封装 [yt-dlp](https://github.com/yt-dlp/yt-dlp)。vYtDL **不重新实现下载器** —— 它调度 yt-dlp 子进程，简化日常参数，并补充播放列表断点续传、TUI 进度和统一下载记录；`serve` 子命令再为 Web / 桌面前端提供 HTTP API + WebSocket + 队列。

本文是完整用法。产品定位与边界见 [`docs/spec.md`](./docs/spec.md)，场景化示例见 [`docs/use-cases.md`](./docs/use-cases.md)。

---

## 安装与构建

```bash
# 本机二进制（Go 1.26+）
go build -trimpath -ldflags "-s -w" -o vYtDL .
# 或: task build

# 交叉编译（macOS / Linux / Windows amd64+arm64）→ dist/
./scripts/build.sh
ONLY=windows ./scripts/build.sh          # 或 task build:windows
# PowerShell: .\scripts\build.ps1 -Only windows

# 内嵌 yt-dlp 的单文件版
./scripts/fetch-ytdlp.sh --embed
go build -tags embed_ytdlp -o vYtDL .
EMBED=1 ONLY=windows ./scripts/build.sh
```

产物：`dist/vYtDL-<os>-<arch>[.exe]`。下载产物（`*.mp4`、`*.part`、`*.ytdl`、记录文件）已被 gitignore。

依赖：

- **yt-dlp**：不是必须预先安装；vYtDL 会自动解析、缓存或下载（见 [yt-dlp 二进制解析](#yt-dlp-二进制解析)）。
- **FFmpeg**：可选。片段裁剪、格式合并、`serve` 的 `extract-audio` 需要它。
- **Node.js / Deno**：YouTube n-challenge 需要 JS 运行时；默认传 `--js-runtimes node`。

---

## 快速开始

```bash
./vYtDL download --install-yt-dlp        # 可选：缓存一份新的 yt-dlp

# YouTube 单视频（1080p，默认 EN+ZH 字幕）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# Bilibili 多 P / 合集（断点续传）
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  "https://www.bilibili.com/video/BVxxxxxx"

# X/Twitter 公开视频：无需额外参数
./vYtDL download --no-tui -o ./downloads \
  "https://x.com/user/status/123"

# 后台服务：随机端口 + 静态托管前端导出
./vYtDL serve --port 0 --static ./out
```

值得了解的默认值：

| 默认 | 值 |
|------|-----|
| 容器 | `-f mp4`（内部为 `--merge-output-format mp4`） |
| 画质 | 空 = 最高（`bestvideo+bestaudio/best`） |
| 字幕 | EN + ZH（含自动字幕） |
| JS 运行时（YouTube） | `--js-runtimes node` |
| 并发 | `-j 1`（串行） |
| 进度 | TUI（`--no-tui` 为纯文本；无 TTY 自动回退） |
| 记录 | `~/.vytdl/download_record.json`（字幕路径在每条记录里） |
| 端口（`serve`） | `0` = 随机空闲端口 |
| 队列并发（`serve`） | `max_concurrent_downloads` 默认 3（范围 1–10） |

---

## 命令总览

| 命令 | 别名 | 说明 |
|------|------|------|
| `download <url…>` | `dl`、`get` | 下载视频 / 播放列表；`--install-yt-dlp` 时只安装 yt-dlp |
| `records` | — | 查看统一下载历史 |
| `analyze <file.vtt>` | `an`、`ana` | VTT → 纯文本 / Markdown |
| `serve` | — | 启动 HTTP API + WebSocket 守护进程 |
| `completion <shell>` | — | Cobra 自动生成的 shell 补全 |

Cobra 根命令的 `Use` 是 `yt-dl`，所以 `--help` 里显示 `yt-dl`；构建出来的二进制名为 `vYtDL`，是同一个程序。

---

## 工作原理

```text
./vYtDL download [flags] URL…
        │
        ▼
  cmd/download.go          解析 flags → downloader.Options；worker pool 分发 URL
        │
        ▼
  internal/ytdlpbin        解析 yt-dlp 二进制
        │
        ▼
  internal/downloader      buildArgs() → exec yt-dlp
        │                    解析 --newline 进度 / --print-json
        │                    可选 playliststate 续传
        ▼
  internal/record          追加到 $VYTDL_HOME/download_record.json
  internal/tui (可选)       Bubble Tea 进度
```

---

## download

```bash
./vYtDL download [flags] <url> [url…]
```

### 参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `-f, --format` | `mp4` | 输出合并容器（mp4 / webm / mkv…），传给 `--merge-output-format`；不做转码 |
| `-q, --quality` | 空 | 画质：`720`、`1080`、`2160`、`best`，或任意 yt-dlp `-f` 选择器；空 = 最高 |
| `-o, --output` | `.` | 输出目录；模板为 `<dir>/%(title)s.%(ext)s` |
| `--start` | 空 | 裁剪起点（`HH:MM:SS` 或秒） |
| `--end` | 空 | 裁剪终点 |
| `--sub-langs` | `en,zh` | 字幕语言（逗号分隔） |
| `--no-subs` | 关 | 关闭字幕（手动 + 自动） |
| `--no-auto-subs` | 关 | 只关闭自动字幕 |
| `-p, --playlist` | 关 | 按播放列表 / 合集处理 |
| `--reset-playlist-state` | 关 | 丢弃续传状态，从头开始 |
| `-j, --concurrency` | `1` | 多个 URL 之间的并发数；播放列表内部仍逐条 |
| `--no-tui` | 关 | 关闭 TUI，打印纯文本进度 |
| `--log-format` | `json` | 历史格式：`json` 或 `csv` |
| `--record-file` | 空 | 历史路径；空 = `$VYTDL_HOME/download_record.json` |
| `--yt-dlp-bin` | `config.json` 值 | yt-dlp / youtube-dl 路径 |
| `--cookies` | 空 | Netscape cookie 文件 |
| `--cookies-from-browser` | 空 | 浏览器 profile 选择器（如 `chrome`） |
| `--proxy` | 空 | HTTP/HTTPS/SOCKS 代理 |
| `--user-agent` | 空 | 自定义 UA |
| `--extractor-args` | 空 | 透传 yt-dlp extractor 参数 |
| `--js-runtimes` | `node` | 传给 `--js-runtimes` |
| `--retries` | `10` | 同时设置 `--retries` 与 `--extractor-retries` |
| `--socket-timeout` | `30` | 网络超时（秒） |
| `--force-ipv4` | 关 | 强制 IPv4 |
| `--install-yt-dlp` | 关 | 下载/更新 yt-dlp 到用户缓存后退出（不需要 URL） |

### 画质 / 容器语义

`-q` 的取值规则（`internal/downloader.buildArgs`）：

| 输入 | 实际 `-f` |
|------|-----------|
| 空、`best`、`bestvideo+bestaudio` | `bestvideo+bestaudio/best` |
| `720`、`1080`、`1080p` | `bestvideo[height<=N]+bestaudio/best[height<=N]` |
| 其他字符串 | 原样作为 `-f` 选择器 |

`-f mp4` 只是合并容器偏好；如果某站点没有可合并进 mp4 的流，yt-dlp 仍可能输出其他容器。

### 字幕

默认：

```bash
--write-subs --write-auto-subs --sub-langs en,zh
```

- `--no-subs`：不请求字幕。
- `--no-auto-subs`：保留手动字幕，不下载自动字幕。
- `--sub-langs zh`：只要中文。
- 字幕文件落在输出目录，路径会写进历史的 `subtitles` 字段。

### 片段裁剪

```bash
./vYtDL download --no-tui -o ./downloads \
  --start 00:01:00 --end 00:02:30 "URL"
```

内部生成：

```bash
--download-sections "*00:01:00-00:02:30" --force-keyframes-at-cuts
```

只给 `--start` 时终点为 `inf`；只给 `--end` 时起点为 `0`。需要 FFmpeg。

### 播放列表 / 断点续传

`--playlist` 流程：

1. 用 `yt-dlp --dump-single-json --flat-playlist` 拉取条目。
2. 创建 `<输出目录>/<播放列表标题>/`（标题做文件系统安全清洗）。
3. 写入 `.playlist_state.json`，逐条下载。
4. 已 `succeeded` 的条目直接跳过（TUI 显示 `skipped`）。
5. 失败的条目下次重试；被中断的 `running` 条目会转为 `failed`。
6. 重跑**相同命令**即续传；`--reset-playlist-state` 丢弃状态。
7. 如果状态里成功的条目文件已被删除，会重新排队。

```text
./downloads/My Playlist/
  .playlist_state.json
  Video One.mp4
  Video One.en.vtt
  …
```

不传 `--playlist` 时，单视频链接会加 `--no-playlist`，避免 YouTube 的 `list=RD…` 被当成整列表。

### 多 URL 并发

```bash
./vYtDL download --no-tui -j 3 -o ./downloads \
  "URL1" "URL2" "URL3"
```

- `-j 1` 串行（默认）；>1 用信号量控制同时在跑的 URL 数。
- 每个 URL 可以是不同站点。
- 播放列表内部的条目始终逐条下载。
- 所有结果在整条命令结束后写入历史；历史写入带锁，避免并发写坏。
- `-j` 越高越容易触发站点限流；建议 1–3。

### 进度与 TUI

默认启动 Bubble Tea TUI（备用屏），显示每条视频的进度条、速度、ETA、完成 / 失败 / 跳过状态。

```bash
./vYtDL download --no-tui ...    # 纯文本：[12.3%] 标题 速度 ETA
```

没有 TTY（脚本、cron、CI、Agent 沙箱）时，TUI 失败会自动回退纯文本，不会阻塞进度通道。

### 历史写入

- `download` 在整条命令结束后把结果追加到历史（`--record-file` / `--log-format` 决定文件）。
- 默认 `~/.vytdl/download_record.json`；`--log-format csv` 时默认 `~/.vytdl/download_record.csv`。
- 只给文件名（不含 `/`）时放在 `$VYTDL_HOME` 下；给绝对路径或含 `/` 的路径时按原路径使用；缺扩展名会自动补 `.json` / `.csv`。
- 结束时会打印 `Download log: <path>` 和 `Completed: X succeeded, Y failed.`；有失败时进程退出码非 0。
- 当前目录（以及 `download` 的输出目录）里的旧 `download_record.*` / `subtitle_mapping.*` 会被并入统一历史，然后删除旧文件。

### 参数 → yt-dlp 映射

| vYtDL 参数 | yt-dlp 等价（近似） |
|-----------|------------------------------|
| `-q 1080` | `-f "bestvideo[height<=1080]+bestaudio/best[height<=1080]"` |
| `-f mp4` | `--merge-output-format mp4` |
| `-o ./dir` | `-o "./dir/%(title)s.%(ext)s"` |
| （默认字幕） | `--write-subs --write-auto-subs --sub-langs en,zh` |
| `--start` / `--end` | `--download-sections "*START-END" --force-keyframes-at-cuts` |
| （默认单视频） | `--no-playlist` |
| `--playlist` | 播放列表模式 + 标题子目录 + `.playlist_state.json` |
| `--cookies-from-browser` | 相同 |
| `--cookies` | 相同 |
| `--proxy` / `--user-agent` | 相同 |
| `--extractor-args` | 相同（X/Twitter 公开链接自动追加 `twitter:api=syndication`） |
| `--js-runtimes node` | 相同（vYtDL 默认） |
| `--retries` / `--socket-timeout` / `--force-ipv4` | 相同 |
| （内部固定） | `--newline --progress --print-json` |

代码路径：`cmd/download.go` → `internal/downloader/options.go` → `buildArgs()`（`internal/downloader/downloader.go`）。

### yt-dlp 二进制解析

顺序（`internal/ytdlpbin`）：

1. `--yt-dlp-bin` / `config.json` `yt_dlp_bin` / `YT_DL_BIN`
2. PATH 上的 `yt-dlp` / `youtube-dl`
3. 内嵌二进制（`-tags embed_ytdlp`）
4. 缓存：OS 用户缓存目录下的 `vYtDL`（macOS `~/Library/Caches/vYtDL`，Linux `~/.cache/vYtDL`，Windows `%LocalAppData%\vYtDL`）
5. GitHub Releases 自动下载

```bash
./vYtDL download --install-yt-dlp
export VYTDL_YTDLP_MIRROR="https://ghproxy.net/https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
```

`config.json` 中 `yt_dlp_bin` 为空表示「自动解析」。仅当需要固定路径时才复制 `config.example.json`：

```json
{ "yt_dlp_bin": "/opt/homebrew/bin/yt-dlp" }
```

配置文件查找顺序：`VYTDL_CONFIG` → 当前目录 `config.json` → 可执行文件目录 `config.json`。

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
  "https://www.youtube.com/watch?v=VIDEO_ID"

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

等价的裸 yt-dlp 命令：

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

续传与产物见 [`docs/use-cases.md#uc-2`](./docs/use-cases.md#uc-2youtube-播放列表断点续传)。

### 反爬 / 403 / n-challenge

YouTube 越来越多地要求 **JS 运行时**来解 `n` 参数挑战。vYtDL 默认传 `--js-runtimes node`。

| 症状 | 处理 |
|---------|-----|
| `No supported JavaScript runtime` / `n challenge solving failed` | 装 Node.js；或 `brew install deno` 后传 `--js-runtimes deno` |
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

---

## Bilibili（哔哩哔哩）

能下。vYtDL 把地址交给本机 yt-dlp，没有单独的 B 站实现，也没有域名白名单。地址全表（稿件、番剧、空间、收藏夹、课堂、直播、国际站）和本机核对记录见 **[docs/bilibili.md](./docs/bilibili.md)**。

### URL 形态

| 类型 | 示例 |
|------|---------|
| 单视频 / 多 P | `https://www.bilibili.com/video/BVxxxxxx` |
| `av` 号 | `https://www.bilibili.com/video/av123456` |
| 番剧 / 季 | `https://www.bilibili.com/bangumi/play/ss…` 或 `ep…` |
| 收藏夹 / 合集 / 空间投稿 | 见 [docs/bilibili.md](./docs/bilibili.md)，用 `--playlist` |

多 P（分 P）视频共用一个 BV 链接。用 `--playlist` 把所有分 P 下到一个标题子目录并支持续传；指定某一 P 时 URL 加 `?p=N` 且不要加 `--playlist`。

### 单视频（单 P）

```bash
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BV1xx411c7mD"
```

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

续传方式与 YouTube 播放列表相同：重跑同一命令；状态在 `./downloads/<标题>/.playlist_state.json`。

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
| 短链 | `b23.tv` 当前 yt-dlp 没有专用 extractor；先打开得到 `bilibili.com` 地址。 |

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

只认带 `/status/<id>` 的单条推文。`https://x.com/<handle>` 和 `/media` 列不出该账号的发帖，也不能按「近 3 个月」筛选：当前 yt-dlp 没有用户时间线 extractor，syndication 按一条推文取视频，CLI 没有日期参数。多个 status 可以一次传入。一条推文里的多段视频给这条链接加 `--playlist`。调用约定见 `skills/x-video-dl/SKILL.md`。

---

## 多站点

同一个 CLI；TikTok / 小红书 / Instagram 通常需要 cookie。

```bash
./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.tiktok.com/@user/video/123"

./vYtDL download --no-tui -o ./downloads --cookies-from-browser chrome \
  "https://www.xiaohongshu.com/explore/xxxxxx"
```

列出全部 extractor：`yt-dlp --list-extractors`。站点能不能下取决于当前 yt-dlp 版本，不是 vYtDL 的硬编码列表。

---

## records

查询 CLI 与 `serve` 共用的统一下载历史。

```bash
./vYtDL records                       # 表格，最新在前
./vYtDL records --json                # 完整 JSON（version + records）
./vYtDL records --failed              # 只看失败
./vYtDL records --limit 20            # 最近 20 条
./vYtDL records --log-format csv      # 读写 CSV 历史
./vYtDL records --record-file /path/to/download_record.json
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `--log-format` | `json` | `json` 或 `csv` |
| `--record-file` | 空 | 历史路径；解析规则同 `download` |
| `--failed` | 关 | 只显示失败记录 |
| `--json` | 关 | 输出 JSON 到 stdout |
| `--limit` | `0` | 只显示最近 N 条（0 = 全部） |

表格列：`STARTED / OK / VIDEO / TITLE / FILE / SUBS`。`Download log: <path>` 打在 stderr；JSON 在 stdout。`--json` 输出：

```json
{
  "version": 1,
  "records": [
    {
      "video_id": "...",
      "title": "...",
      "url": "...",
      "output_dir": "...",
      "filename": "...",
      "subtitles": ["..."],
      "success": true,
      "error": "",
      "started_at": "2026-11-10T...",
      "finished_at": "2026-11-10T...",
      "duration": "1m2s"
    }
  ]
}
```

字段定义见 [`docs/spec.md#5-数据与状态契约`](./docs/spec.md#5-数据与状态契约)。`records` 也会把当前目录里的旧 `download_record.*` / `subtitle_mapping.*` 并入统一历史后删除旧文件。

---

## analyze

```bash
./vYtDL analyze --mode text video.en.vtt
./vYtDL analyze --mode markdown --output transcript.md video.zh.vtt
./vYtDL analyze < video.vtt          # 从 stdin 读
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `-m, --mode` | `text` | `text`：纯文本；`markdown`：带时间戳的 Markdown |
| `-o, --output` | stdout | 输出文件 |
| `-s, --segment` | 空 | 保留参数；当前没有分段输出模式，设置后不生效 |

- 支持 YouTube 自动生成 VTT（`<c>` 标签），会去除重复的 10ms 快照 cue。
- Markdown 结构：`# Transcript`，按小时插入 `## HH:MM:SS`，每句前带 `HH:MM:SS`。
- 别名：`an`、`ana`。

---

## 后台服务（serve）

`serve` 启动 vYtDL daemon：HTTP API + WebSocket 服务，负责下载队列、SQLite 持久化、VTT 分析，可选静态托管前端导出。它是 Web / 桌面前端的后端。

```bash
# 桌面壳模式：随机回环端口，托管前端导出目录
./vYtDL serve --port 0 --static ./out

# Docker 部署：全接口 + 固定端口
./vYtDL serve --host 0.0.0.0 --port 3000 --static /app/out --home /app/data

# 本地开发：请求日志
./vYtDL serve --dev --home ./.daemon-home
```

### 参数与环境变量

| 参数 | 默认 | 环境变量 | 含义 |
|------|------|----------|------|
| `--port` | `0` | `PORT` | 监听端口；0 = 随机空闲端口 |
| `--host` | `127.0.0.1` | `VYTDL_HOST` | 绑定地址；Docker 用 `0.0.0.0` |
| `--static` | 空 | `VYTDL_STATIC_DIR` | 从该目录托管前端导出（含 SPA 回退） |
| `--home` | `~/.vytdl` | `VYTDL_HOME` | 数据根目录：`vytdl.db`、下载历史 |
| `--dev` | 关 | — | 请求日志 |

下载默认目录为 `<home>/downloads`，可用 `VYTDL_OUTPUT_DIR` 覆盖（或在前端设置 `default_output_dir`）。

启动时会打印：

```text
vYtDL daemon 1.0.0 listening on http://127.0.0.1:PORT
  data root: ...
  downloads: ...
  static:    ...        # 配置了 --static 时
```

### 行为

- 启动时恢复上次停止时处于 `pending` / `downloading` 状态的下载（参数存在 `downloads.options` 列）。
- 队列上限来自设置 `max_concurrent_downloads`（默认 3，范围 1–10），修改后立即生效。
- 取消可作用于排队中或进行中的下载；删除会先取消再删记录。
- 重试会以单条重新入队（不复用原播放列表模式）。
- 每完成一次下载（成功 / 失败 / 取消），结果追加到 `<home>/download_record.json`，与 CLI 的 `vYtDL records` 是同一份历史。
- `GET /api/downloads` 读的是 SQLite 队列 `vytdl.db`，不是下载历史文件。
- 暂停 / 恢复不支持：`/api/pause-download`、`/api/resume-download` 返回 `success:false`（HTTP 200）。
- 静态托管只接受 GET/HEAD；找不到的路径回退到 `index.html`。
- `--static` 目录不存在时启动失败。
- 收到 SIGINT / SIGTERM 时取消进行中的下载并优雅退出。

### API 统一响应

```json
{"success": true, "data": ...}
{"success": false, "error": "..."}
```

CORS 允许所有来源（绑地址默认仅回环；Docker 部署时自行控制网络暴露）。

### 端点

| 方法 | 路径 | 作用 |
|------|------|------|
| GET | `/api/health` | 健康检查：`status / service / version` |
| GET | `/api/ws` | WebSocket 连接（进度 / 队列 / VTT 事件） |
| POST | `/api/start-download` | 新建下载并入队，返回下载 ID |
| POST | `/api/cancel-download` | 取消（`{"downloadId": "..."}`） |
| POST | `/api/retry-download` | 重试（`{"id": "..."}`），返回新 ID |
| POST | `/api/get-downloads` | 列出队列记录 |
| GET | `/api/downloads` | 同上（无请求体） |
| POST | `/api/delete-download` | 删除记录（`{"id": "..."}`） |
| POST | `/api/get-video-info` | 拉取视频元数据与格式（`{"url": "..."}`） |
| POST | `/api/get-video-formats` | 只返回格式列表 |
| POST | `/api/get-playlist-info` | 拉取播放列表扁平条目 |
| POST | `/api/get-settings` | 读取设置 |
| POST | `/api/update-settings` | 更新设置（`{"settings": {...}}`） |
| POST | `/api/get-default-output-dir` | 默认输出目录 |
| POST | `/api/pause-download` | 不支持，返回 `success:false` |
| POST | `/api/resume-download` | 不支持，返回 `success:false` |
| POST | `/api/extract-audio` | FFmpeg 提取音频（`video_path` / `output_dir` / `audio_format`） |
| POST | `/api/analyze-vtt` | 后台分析 VTT（`{"url": "..."}`），返回 `reportId` |
| GET | `/api/vtt-reports` | 报告列表（`page` / `limit` / `lang`） |
| GET | `/api/vtt-report/{id}` | 单份报告 |
| POST | `/api/delete-vtt-report` | 删除报告（`{"id": "..."}`） |
| POST | `/api/open-download-folder` | 用系统文件管理器打开目录（`{"path": "..."}`） |
| POST | `/api/reveal-in-folder` | 在系统文件管理器中定位文件 |

`/api/start-download` 的请求体可以是 `{"request": {...}}`（前端 Tauri 镜像写法）或扁平对象。可识别字段：

```text
url, title, collection_id, collection_title, is_playlist,
quality, format_id, format, output_dir, sub_langs,
write_subs, write_auto_subs, start_time, end_time,
cookie {mode: "file"|"browser", path, browser},
proxy, rate_limit, concurrent_fragments,
embed_thumbnail, embed_metadata, embed_chapters, sponsorblock_remove,
filename_template, po_token, extractor_args, config_location
```

服务模式（`serve`）会：默认忽略用户级 yt-dlp 配置（`--ignore-config`，除非传 `config_location`）；强制 `--windows-filenames`；自动字幕加 `--sleep-subtitles 1`；片段裁剪使用 muxed-safe 选择器并关闭 direct merge；X/Twitter 自动 syndication。

### WebSocket 事件

连接 `/api/ws`，daemon 只发送、不要求客户端消息：

```json
{"event": "download:progress:<id>", "data": {...}}
{"event": "download:status:<id>", "data": "pending|downloading|completed|failed|cancelled"}
{"event": "download:complete:<id>", "data": {...}}
{"event": "download:error:<id>", "data": {...}}
{"event": "download:log:<id>", "data": {"level": "...", "message": "..."}}
{"event": "queue:update", "data": {"id": "...", "status": "..."}}
{"type": "vtt-report:status", "reportId": "...", "status": "pending|processing|failed"}
{"type": "vtt-report:complete", "reportId": "..."}
```

### 设置键

| 键 | 默认 | 说明 |
|----|------|------|
| `yt_dlp_path` | 解析到的 yt-dlp 路径 | 服务使用的二进制 |
| `default_output_dir` | `<home>/downloads` | 默认下载目录 |
| `default_quality` | `best` | 默认画质 |
| `default_format` | `mp4` | 默认容器 |
| `default_sub_langs` | `["en","zh"]` | 默认字幕语言（JSON 数组） |
| `language` | `zh` | 前端语言 |
| `max_concurrent_downloads` | `3` | 队列并发（1–10） |
| `ai_provider` / `ai_api_key` / `ai_model` | 空 | 前端 AI 配置存储位 |
| `agent_cli_kimi_bin` / `agent_cli_other_bin` | 空 | 前端 Agent CLI 配置存储位 |

### VTT 分析与音频提取

- `POST /api/analyze-vtt` 会依次尝试字幕语言 `zh → en → ja → ko → zh-Hans → zh-Hant`，下载到 `<输出目录>/vtt-temp/<reportId>`，解析成 Markdown 后存进 `vtt_reports`，完成后清理临时目录。
- `POST /api/extract-audio` 调 FFmpeg；`audio_format` 支持 `mp3`（默认）、`m4a`/`aac`、`flac`、`wav`、`ogg`、`opus`。FFmpeg 路径可用 `FFMPEG_PATH` 覆盖。
- 打开 / 定位目录在 Docker 里会失败，返回 `success:false`（这是预期行为，前端会提示）。

---

## 输出文件

下载历史只有一份，CLI 和 `serve` 都往里追加。字幕路径是每条记录的 `subtitles` 字段。

| 文件 | 用途 |
|------|------|
| `~/.vytdl/download_record.json`（或 `.csv`） | 成功/失败、URL、视频路径、字幕路径、错误、耗时 |
| `$VYTDL_HOME/vytdl.db` | 仅 `serve` 的实时队列、设置、VTT 报告；不是下载历史 |
| `<playlist>/.playlist_state.json` | 播放列表续传队列 |

```bash
./vYtDL records                 # 查看历史
./vYtDL records --json          # 机器可读，含字幕路径
./vYtDL records --failed
```

默认目录是 `~/.vytdl`（与 `serve --home` 相同，可用 `VYTDL_HOME` 覆盖）。`--record-file` 改路径；只写文件名时仍放在该目录下。`--log-format csv` 换成单个 CSV，字幕列用 `|` 分隔。

当前目录里如果还有旧的 `download_record.*` 和 `subtitle_mapping.*`，`download` 或 `records` 会把它们并进这份历史，然后删除旧文件。

---

## Shell 封装与批量脚本

```bash
# 单视频 / 合集（内部用 go run .，参数固定）
./scripts/download_video.sh "https://www.youtube.com/watch?v=ID" ./downloads 1080
./scripts/download_collection.sh "https://www.youtube.com/playlist?list=PLxxxx" ./downloads 720

# Python 批量：文件里一行一个 URL，# 开头为注释
python3 batch_download.py urls.txt ./downloads
python3 batch_download.py urls.txt ./downloads -q 1080 --playlist
python3 batch_download.py urls.txt ./downloads --log-format csv
```

`batch_download.py` 会查找当前目录或 PATH 里的 `vYtDL`，逐条调用 `download`，最后汇总成功 / 失败数。它接受任意 yt-dlp 支持的 URL，不限于 YouTube。

---

## 环境变量速查

| 变量 | 作用 | 适用 |
|------|------|------|
| `VYTDL_HOME` | 数据根目录（下载历史、`serve` 的 `vytdl.db`） | CLI / serve |
| `VYTDL_CONFIG` | `config.json` 路径 | CLI |
| `YT_DL_BIN` | yt-dlp 路径（等价于 `--yt-dlp-bin`） | CLI / serve（serve 优先读设置里的 `yt_dlp_path`） |
| `VYTDL_YTDLP_MIRROR` | yt-dlp 下载镜像 | CLI / 脚本 |
| `VYTDL_HOST` / `PORT` / `VYTDL_STATIC_DIR` | serve 的 host / port / 静态目录默认值 | serve |
| `VYTDL_OUTPUT_DIR` | serve 默认下载目录 | serve |
| `FFMPEG_PATH` | FFmpeg 路径 | serve 音频提取 |
| `HTTP(S)_PROXY` / `ALL_PROXY` / `NO_PROXY` | 网络代理；`serve` 的元数据 / VTT 调用会过滤非标准 `*_proxy` 变量，下载子进程直接继承环境 | 两者 |

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
| TUI 在脚本 / Agent 里报错 | 加 `--no-tui`；无 TTY 时也会自动回退 |
| `records` 里没有刚下的视频 | 确认命令已结束（历史在整条命令结束时写入）；确认 `--record-file` / `VYTDL_HOME` 一致 |
| `serve` 启动报静态目录不存在 | 去掉 `--static` 或给出存在的导出目录 |
| Docker 里「打开目录」失败 | 预期行为，容器内无法调宿主文件管理器 |

---

## 文档索引

| 文档 | 内容 |
|------|------|
| [`README.md`](./README.md) | 项目入口、快速开始 |
| [`docs/spec.md`](./docs/spec.md) | 用途与能力边界规范 |
| [`docs/use-cases.md`](./docs/use-cases.md) | YouTube / B 站 / X / 多站点 / serve / Skill 用例 |
| [`docs/bilibili.md`](./docs/bilibili.md) | B 站可下载范围与限制 |
| [`docs/skills-guide.md`](./docs/skills-guide.md) | Skill 安装与各 Agent 工具用法 |
| [`skills/README.md`](./skills/README.md) | Skill 总览 |
