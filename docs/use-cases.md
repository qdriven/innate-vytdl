# vYtDL 使用用例（Use Cases）

本文用**端到端场景**说明 vYtDL 怎么用。每个用例包含：场景 → 命令 → 预期产物 → 验证方式 → 常见失败处理。参数细节与完整命令面见 [`USAGE.md`](../USAGE.md)，能力边界见 [`docs/spec.md`](./spec.md)。

速查：

| 我想… | 用哪个 |
|-------|--------|
| 下 YouTube 单视频 / 片段 | [UC-1](#uc-1youtube-单视频--字幕--片段) |
| 下 YouTube 播放列表，断了能续 | [UC-2](#uc-2youtube-播放列表断点续传) |
| 下 B 站多 P / 番剧 / 合集 | [UC-3](#uc-3b-站多-p--番剧--合集) |
| 下一条 X/Twitter 推文视频 | [UC-4](#uc-4xtwitter-单条推文) |
| 一次下多个站点 / 多个链接 | [UC-5](#uc-5多站点批量与并发) |
| 把字幕转成文本 / Markdown | [UC-6](#uc-6vtt-字幕转文本) |
| 确认「下过没有、字幕在哪、为什么失败」 | [UC-7](#uc-7查询下载历史) |
| 给 Web / 桌面端起后端 | [UC-8](#uc-8启动-serve-后端) |
| 让 AI Agent 自动按规范下载 | [UC-9](#uc-9用-agent-skill-执行下载) |

## 前置条件

```bash
# 1. 构建（Go 1.26+）
go build -o vYtDL .

# 2. 确认/安装 yt-dlp（三种方式任选）
yt-dlp --version                       # 已在 PATH
./vYtDL download --install-yt-dlp      # 缓存到用户目录，无需系统安装
# 或构建时内嵌：./scripts/fetch-ytdlp.sh --embed && go build -tags embed_ytdlp -o vYtDL .

# 3. 可选：FFmpeg（片段裁剪、合并、音频提取、serve 的 extract-audio）
ffmpeg -version
```

约定：文档里的 `VIDEO_ID`、`BVxxxxxx`、`123` 都是占位符，替换成真实值。所有命令在仓库根目录执行。

---

## UC-1：YouTube 单视频 + 字幕 + 片段

**场景**：下载一个 YouTube 视频，1080p 以内最高画质，带英文/中文字幕；或者只截取其中一段。

```bash
# 1080p + 默认 EN/ZH 字幕（含自动字幕）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# 只截取 00:01:00–00:02:30（需要 FFmpeg）
./vYtDL download --no-tui -o ./downloads \
  --start 00:01:00 --end 00:02:30 \
  "https://www.youtube.com/watch?v=VIDEO_ID"

# 不要字幕
./vYtDL download --no-tui -o ./downloads --no-subs \
  "https://www.youtube.com/watch?v=VIDEO_ID"
```

**预期产物**：

```text
./downloads/<标题>.mp4
./downloads/<标题>.en.vtt
./downloads/<标题>.zh.vtt
~/.vytdl/download_record.json      # 每次下载都追加一条
```

**验证**：

```bash
./vYtDL records --json --limit 1    # 看 filename 和 subtitles
```

**常见失败**：

| 症状 | 处理 |
|------|------|
| `No supported JavaScript runtime` / n-challenge | 装 Node.js；或 `--js-runtimes deno` |
| `Sign in to confirm you're not a bot` / 403 | `--cookies-from-browser chrome`；更新 yt-dlp；必要时 `--extractor-args "youtube:player_client=web,android"` |
| 片段裁剪失败 | 确认 `ffmpeg` 在 PATH |
| 代理报错 | 检查 `HTTPS_PROXY`，或显式 `--proxy`；必要时 `--force-ipv4` |

---

## UC-2：YouTube 播放列表断点续传

**场景**：下载整个播放列表；中途断网 / 关机后，重跑同一条命令只补没下完的。

```bash
./vYtDL download --no-tui --playlist -o ./downloads -q 720 \
  "https://www.youtube.com/playlist?list=PLxxxx"
```

**预期产物**：

```text
./downloads/<播放列表标题>/
  .playlist_state.json          # 逐条状态：succeeded / failed / pending
  <视频标题>.mp4
  <视频标题>.en.vtt
  …
```

**验证 / 续传**：

```bash
# 中断后，重跑完全相同的命令即可
./vYtDL download --no-tui --playlist -o ./downloads -q 720 \
  "https://www.youtube.com/playlist?list=PLxxxx"
# 已成功的条目显示 skipped，不会重复下载

# 想彻底重来：
./vYtDL download --no-tui --playlist --reset-playlist-state -o ./downloads \
  "https://www.youtube.com/playlist?list=PLxxxx"
```

**注意**：

- 续传依赖**相同的输出目录 + 相同的播放列表 URL**；改了 `-o` 会换一个状态文件。
- 如果状态里标记成功但文件被删了，下次运行会重新排队。
- 播放列表内部是逐条下载；`-j` 只作用于多个 URL 参数之间。

---

## UC-3：B 站多 P / 番剧 / 合集

**场景**：下载 B 站稿件、多分 P、番剧季、收藏夹或空间投稿，并支持续传。B 站高画质和 CC 字幕通常需要已登录 Cookie。

```bash
# 当前这一 P（默认 --no-playlist）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# 全部分 P / 番剧整季 / 收藏夹 / 合集 / 空间投稿
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/bangumi/play/ss12345"

# 指定第 2 P：URL 带 ?p=2，且不要加 --playlist
./vYtDL download --no-tui -o ./downloads -q 1080 \
  "https://www.bilibili.com/video/BVxxxxxx?p=2"
```

**预期产物**：与 YouTube 播放列表相同：`./downloads/<标题>/` + `.playlist_state.json` + 成片/字幕。

**验证**：

```bash
./vYtDL records --json --limit 5
```

**已知限制**（详见 [`docs/bilibili.md`](./bilibili.md)）：

- `b23.tv` 短链当前 yt-dlp 没有专用 extractor：先在浏览器打开，改用跳转后的 `bilibili.com` 地址。
- `-q 1080` 只是高度上限；实际画质取决于登录态 / 大会员。
- 大会员专享正片、未购买的课堂课节下不了。
- 别人的私密收藏夹、稍后再看必须用列表所有者的 Cookie。

---

## UC-4：X/Twitter 单条推文

**场景**：下载一条推文里的视频。公开推文开箱即用；受保护 / 敏感内容需要浏览器 Cookie。

```bash
# 公开推文：自动走 syndication API，无需 cookie
./vYtDL download --no-tui -o ./downloads \
  "https://x.com/user/status/123"

# 私密 / 受限推文：登录 Cookie + 显式 GraphQL（会关闭 syndication 回退）
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  --extractor-args "twitter:api=graphql" \
  "https://x.com/user/status/123"
```

**支持范围**：

- 只认单条 `/status/<id>`（`x.com`、`twitter.com` 及其子域、`vxtwitter.com`、`fxtwitter.com`、`nitter.net`）。
- 一条推文里有多段视频时，给这条 status 加 `--playlist`。
- **不支持**按 handle / 主页 / `/media` 批量枚举，也不支持「近 3 个月」筛选；CLI 没有日期参数。

**验证**：

```bash
./vYtDL records --json --limit 1
# 或直接检查 ./downloads 下的 mp4
```

**常见失败**：

| 症状 | 处理 |
|------|------|
| 公开推文拿不到格式 | 更新 yt-dlp；确认链接是 `/status/<id>` |
| 受保护内容 `Unsupported URL` | 确认浏览器已登录 x.com；换浏览器或 `--cookies cookies.txt` |
| 下到的是图片 / 纯文本 | 该推文本体没有视频，用 `yt-dlp --list-formats <链接>` 确认 |

轻量替代方案：`skills/x-video-dl/scripts/x_dl.sh`（直接调 yt-dlp + Cookie，不起 Go 程序）。

---

## UC-5：多站点批量与并发

**场景**：一次给多个链接（可跨站点），最多 `N` 个同时下载。

```bash
./vYtDL download --no-tui -j 3 -o ./downloads \
  "https://www.youtube.com/watch?v=VIDEO_ID" \
  "https://www.bilibili.com/video/BVxxxxxx" \
  "https://x.com/user/status/123" \
  "https://www.tiktok.com/@user/video/1234" \
  "https://www.xiaohongshu.com/explore/xxxxxx"
```

**预期产物**：每条链接各自产生成片/字幕；所有结果在同一次命令结束时写入同一份历史。

**注意**：

- `-j` 越大越容易触发站点的 bot / 限流检查；建议 1–3。
- TikTok、小红书、Instagram 等通常需要 `--cookies-from-browser chrome` 或 `--cookies cookies.txt`。
- 播放列表模式是逐个 URL 参数串行处理；`-j` 不会把一个播放列表拆成并发条目。

**验证**：

```bash
./vYtDL records --limit 10
./vYtDL records --failed          # 只看失败的
```

---

## UC-6：VTT 字幕转文本

**场景**：下载了字幕，想把 VTT 变成可读文本或带时间戳的 Markdown。

```bash
./vYtDL analyze --mode text video.zh.vtt
./vYtDL analyze --mode markdown --output transcript.md video.en.vtt
./vYtDL analyze < video.en.vtt          # 从 stdin 读
```

**预期产物**：

- `text`（默认）：纯文本，句间以空格连接，不带时间戳。
- `markdown`：`# Transcript` + 按小时分组的 `## HH:MM:SS` 标题 + 每句带时间戳。
- `--output` 未给时写 stdout。

**注意**：

- 解析器支持 YouTube 自动字幕的 `<c>` 单词级时间标签，会去掉重复的 10ms 快照 cue。
- `--segment` 参数目前**没有生效**（保留参数位，当前没有按时间分段输出的模式）。
- `serve` 的 VTT 分析（`POST /api/analyze-vtt`）复用同一个解析器，语言顺序为 zh → en → ja → ko → zh-Hans → zh-Hant。

---

## UC-7：查询下载历史

**场景**：确认某个视频下过没有、成片在哪、字幕在哪、为什么失败。

```bash
./vYtDL records                       # 表格，最新在前
./vYtDL records --json --limit 5      # 完整 JSON（含 subtitles）
./vYtDL records --failed              # 只看失败
./vYtDL records --record-file /path/to/download_record.json
```

**输出说明**：

- 表格列：`STARTED / OK / VIDEO / TITLE / FILE / SUBS`。
- 路径打在 stderr：`Download log: …`；JSON 在 stdout。
- `--json` 输出 `{"version": 1, "records": [...]}`。

**要点**：

- 历史是 `$VYTDL_HOME/download_record.json`（默认 `~/.vytdl/download_record.json`），CLI 与 `serve` 共用。
- `serve` 的 `vytdl.db` 是实时队列，不是历史；`.playlist_state.json` 是续传状态，也不是历史。
- 同一 URL 允许多条记录：失败一次、后来成功一次，会有两条；用 `started_at` 区分。

Agent 侧封装：`skills/download-record/SKILL.md`。

---

## UC-8：启动 serve 后端

**场景**：给 Web / 桌面端提供后端，或部署到 Docker。

```bash
# 本地：随机端口 + 托管前端导出目录
./vYtDL serve --port 0 --static ./out

# 固定端口 + 局域网 / Docker
./vYtDL serve --host 0.0.0.0 --port 3000 --static /app/out --home /app/data

# 只提供 API（不托管静态文件）
./vYtDL serve --port 3000 --dev
```

**预期产物 / 行为**：

```text
<home>/vytdl.db                 # downloads / settings / vtt_reports
<home>/download_record.json     # 与 CLI 共用的下载历史
<home>/downloads/               # 默认下载目录（VYTDL_OUTPUT_DIR / 设置可改）
```

**验证**：

```bash
curl http://127.0.0.1:3000/api/health
# {"success":true,"data":{"status":"ok","service":"vytdl-daemon","version":"1.0.0"}}
```

**行为说明**：

- 启动时恢复上次 `pending` / `downloading` 的下载。
- 支持取消、重试（重试按单条重新入队）、删除（先取消再删记录）。
- 暂停 / 恢复不支持：对应端点返回 `success:false`，前端会给出提示。
- 一个下载（含播放列表）结束后，结果追加到 `<home>/download_record.json`。
- 完整端点、请求字段、WebSocket 事件、设置键见 [`USAGE.md`](../USAGE.md#后台服务serve)。

---

## UC-9：用 Agent Skill 执行下载

**场景**：不想手写参数，让 AI Agent 按项目规范完成下载、核对结果。

前置：把 `skills/` 下的目录安装到 Agent 工具（见 [`docs/skills-guide.md`](./skills-guide.md)）。

### 9.1 `vytdl-cli`：通用下载 / 构建 / 排障

```text
用户：用 vYtDL 把这个 B 站合集按 720p 下到 ./downloads，断了要能续：
     https://www.bilibili.com/video/BVxxxxxx
Agent（加载 skills/vytdl-cli/SKILL.md）：
     ./vYtDL download --no-tui --playlist -o ./downloads -q 720 "<URL>"
```

### 9.2 `x-video-dl`：单条推文快速下载

```text
用户：把这个推文的视频存下来：https://x.com/user/status/123
Agent（加载 skills/x-video-dl/SKILL.md）：
     skills/x-video-dl/scripts/x_dl.sh "https://x.com/user/status/123"
     然后用 ffprobe 报告分辨率 / 时长 / 大小
```

### 9.3 `download-record`：交付前核对

```text
用户：刚才那个视频下成功了吗？字幕在哪？
Agent（加载 skills/download-record/SKILL.md）：
     ./vYtDL records --json --limit 3
     → 汇报 filename / subtitles / success / error
```

**协作关系**：

```text
用户自然语言 + 链接
   │  description 匹配
   ▼
Skill（怎么做：流程、参数、排障）
   │
   ▼
vYtDL / x_dl.sh（做什么：实际下载）
   │
   ▼
yt-dlp 子进程
   │
   ▼
download-record（核对 filename / subtitles）→ 汇报
```

三个 Skill 的触发词、安装路径和工具差异见 [`docs/skills-guide.md`](./skills-guide.md)。

---

## UC-10：内嵌 yt-dlp 的单文件分发

**场景**：目标机器不能联网或不想装 yt-dlp，交付一个自带 yt-dlp 的二进制。

```bash
./scripts/fetch-ytdlp.sh --embed          # → internal/ytdlpbin/binaries/yt-dlp.bin
go build -tags embed_ytdlp -o vYtDL .
./vYtDL download --no-tui -o ./downloads "https://www.youtube.com/watch?v=VIDEO_ID"
```

**说明**：

- 内嵌二进制在解析顺序里排在 PATH 之后、缓存之前。
- 交叉编译带内嵌：`EMBED=1 ONLY=windows ./scripts/build.sh`，产物在 `dist/`。
- GitHub 下载慢时可用 `VYTDL_YTDLP_MIRROR` 指向镜像。
