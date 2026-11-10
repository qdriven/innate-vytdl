# vYtDL CLI Spec

本文件是 **vYtDL 的用途与能力边界规范**：说明这个 CLI 做什么、不做什么、有哪些命令和数据契约。它与 `README.md`（入口）、`USAGE.md`（完整用法）、`docs/use-cases.md`（场景用例）构成一套文档。

> 本仓库是独立 Go 项目（module `github.com/innate/yt-dl`），不依赖任何其他仓库。
> 文档与代码冲突时以代码为准，并应回改本文。

## 1. 一句话定位

vYtDL 是一个 **Go CLI / TUI + 后台守护进程**，把 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 作为子进程调度，用来：

- 下载 yt-dlp 支持的站点视频（1800+ extractor：YouTube、Bilibili、TikTok、X/Twitter、Instagram、Vimeo、Twitch、小红书…）；
- 用更短的参数完成常用动作（`-q 1080`、默认 EN/ZH 字幕、`-o` 目录）；
- 为播放列表 / 多 P / 合集提供断点续传；
- 把每次下载写进一份统一历史（含字幕路径）；
- 通过 `vYtDL serve` 为 Web / 桌面前端提供 HTTP API + WebSocket + 下载队列；
- 把 VTT 字幕转成文本 / Markdown。

**它不是独立下载引擎，也不重新实现鉴权。** 站点覆盖和解析能力 = 本机 yt-dlp 的能力。

## 2. 目标（Goals）

| 编号 | 目标 | 落地位置 |
|------|------|----------|
| G1 | 一条命令下载一个或多个 URL（视频 / 片段） | `cmd/download.go`、`internal/downloader` |
| G2 | 播放列表 / 多 P 断点续传，重跑即跳过已完成项 | `internal/downloader`、`internal/playliststate` |
| G3 | 下载历史只有一份，字幕路径写进记录 | `internal/record`、`cmd/records.go` |
| G4 | `serve` 作为 Web / 桌面端唯一后端（队列、SQLite、WS、API） | `internal/server` |
| G5 | 本地 VTT 解析与导出 | `internal/vtt`、`cmd/analyze.go` |
| G6 | 把「怎么下」打包成 Agent Skill，供 AI Agent 自动加载 | `skills/` |
| G7 | yt-dlp 缺失时能自动安装 / 内嵌，降低部署门槛 | `internal/ytdlpbin` |

## 3. 非目标（Non-Goals）

1. **不重新实现下载器**：解析、格式选择、鉴权仍由 yt-dlp 完成。
2. **不重新实现登录 / 签名算法**：cookie、proxy、extractor-args 原样透传。
3. **不维护 URL 域名白名单**：能否下载由当前 yt-dlp 的 extractor 决定。
4. **不创建第二份记录 / 映射文件**：不写 `subtitle_mapping.json`；字幕路径是 `download_record.json` 每条记录里的字段。
5. **不做 X/Twitter 账号时间线枚举**：只认 `/status/<id>` 单条推文；不按 handle 或「近 N 个月」批量抓取。
6. **不在本仓库实现 Web / 桌面 UI**：`serve` 只提供后端和可选静态托管；前端是独立产物。
7. **不做视频转码**：`--format` 只作为 yt-dlp 的合并容器偏好（`--merge-output-format`），不负责编码转换。

## 4. 命令面（Command Surface）

构建产物的二进制名为 `vYtDL`；Cobra 根命令的 `Use` 是 `yt-dl`，因此 `--help` 里显示 `yt-dl`。二者是同一个程序。

| 命令 | 别名 | 用途 | 主要输出 |
|------|------|------|----------|
| `download <url…>` | `dl`、`get` | 下载视频 / 播放列表；`--install-yt-dlp` 时只安装 yt-dlp | 视频、字幕、下载历史 |
| `records` | — | 查询统一下载历史（表格或 JSON） | stdout 历史表 / JSON |
| `analyze <file.vtt>` | `an`、`ana` | VTT → 纯文本 / Markdown（可读 stdin） | stdout 或 `--output` 文件 |
| `serve` | — | 启动 HTTP API + WebSocket 守护进程 | 监听地址、数据目录 |
| `completion <shell>` | — | Cobra 自动生成的 shell 补全 | 补全脚本 |

### 4.1 `download` 的核心参数契约

| 参数 | 默认 | 行为 |
|------|------|------|
| `-f, --format` | `mp4` | 传给 yt-dlp `--merge-output-format` |
| `-q, --quality` | 空（最高） | 数字/`Np` → 高度上限；`best` → 最高；其他字符串按 yt-dlp `-f` 原样使用 |
| `-o, --output` | `.` | 输出目录；内部模板为 `<dir>/%(title)s.%(ext)s` |
| `-p, --playlist` | 关 | 开启播放列表模式（标题子目录 + 续传状态） |
| `--sub-langs` | `en,zh` | 字幕语言列表 |
| `--no-subs` | 关 | 关闭手动 + 自动字幕 |
| `--no-auto-subs` | 关 | 只关闭自动字幕 |
| `--start` / `--end` | 空 | `--download-sections "*START-END"` + `--force-keyframes-at-cuts`（需 FFmpeg） |
| `--cookies` / `--cookies-from-browser` | 空 | 原样传给 yt-dlp |
| `--proxy` / `--user-agent` | 空 | 原样传给 yt-dlp |
| `--extractor-args` | 空 | 原样传给 yt-dlp；X/Twitter 公开链接会自动追加 `twitter:api=syndication` |
| `--js-runtimes` | `node` | 传给 yt-dlp（YouTube n-challenge） |
| `--retries` | `10` | 同时设置 `--retries` 与 `--extractor-retries` |
| `--socket-timeout` | `30` | 传给 yt-dlp（秒） |
| `--force-ipv4` | 关 | 传给 yt-dlp |
| `-j, --concurrency` | `1` | 多个 URL 参数之间的并发数；播放列表内部仍逐条下载 |
| `--record-file` | `$VYTDL_HOME/download_record.json` | 历史文件路径；只给文件名时放在数据目录 |
| `--log-format` | `json` | `json` 或 `csv`；同时决定默认历史文件名 |
| `--reset-playlist-state` | 关 | 丢弃续传状态，从头开始 |
| `--no-tui` | 关 | 关闭 Bubble Tea TUI，改打印纯文本进度 |
| `--yt-dlp-bin` | `config.json` 值 | 指定 yt-dlp / youtube-dl 路径 |
| `--install-yt-dlp` | 关 | 下载/更新 yt-dlp 到用户缓存后退出 |

### 4.2 默认行为

- 默认字幕：`--write-subs --write-auto-subs --sub-langs en,zh`。
- 默认非播放列表：对单视频链接追加 `--no-playlist`，避免 `list=RD…` 之类被当成整列表。
- 默认进度：交互式 TUI；无 TTY 时自动回退纯文本，不会让下载卡住。
- 默认记录：所有结果下载结束后追加到同一份历史文件（加锁写入）。
- 默认二进制解析：`--yt-dlp-bin` / `config.json` / `YT_DL_BIN` → PATH 的 `yt-dlp` / `youtube-dl` → 内嵌（`-tags embed_ytdlp`）→ 用户缓存 → GitHub Releases 自动下载。

## 5. 数据与状态契约

| 文件 / 目录 | 所有者 | 内容 | 不要做什么 |
|-------------|--------|------|------------|
| `$VYTDL_HOME/download_record.json`（默认 `~/.vytdl/download_record.json`） | CLI + `serve` | 统一历史：`version: 1` + `records[]`，字段见下 | 不要另建 `subtitle_mapping.json` |
| `$VYTDL_HOME/download_record.csv` | CLI（`--log-format csv`） | 同一份历史的 CSV 形式，字幕用 `\|` 分隔 | 不要把它当第二份历史 |
| `$VYTDL_HOME/vytdl.db` | 仅 `serve` | SQLite：`downloads`（实时队列）、`settings`、`vtt_reports` | 不要用它回答「历史上下过什么」 |
| `<输出目录>/<播放列表标题>/.playlist_state.json` | CLI | 播放列表逐条状态（pending/running/succeeded/failed、文件、字幕、错误） | 不要当下载历史 |
| `<输出目录>/<标题>`（播放列表） | CLI | 该播放列表的成片与字幕 | — |

历史记录字段（`internal/record/record.go`）：

```
video_id, title, url, output_dir, filename, subtitles[],
success, error, started_at, finished_at, duration
```

CSV 表头与字段一一对应；旧版没有 `subtitles` 列的 CSV 也能导入。同一 URL 允许多条记录（失败后再成功），用 `started_at` 区分。

旧格式 `download_record.*` + `subtitle_mapping.*` 若出现在当前目录（`download` 还会检查输出目录），会被并入统一历史，然后删除旧文件（活动历史文件本身除外）。

## 6. `serve` 契约

- 默认只绑定 `127.0.0.1`；`--host 0.0.0.0` 用于 Docker。
- 默认端口 `0`（随机空闲端口，桌面壳就绪探测用）；`PORT` 环境变量可给默认值。
- 数据根 `--home`（默认 `~/.vytdl`，`VYTDL_HOME` 可覆盖）；下载默认目录 `<home>/downloads`（`VYTDL_OUTPUT_DIR` 可覆盖）。
- 队列并发上限来自设置 `max_concurrent_downloads`（默认 3，范围 1–10）。
- 启动时恢复上次处于 `pending` / `downloading` 的下载；序列化参数存在 `downloads.options` 列。
- 每次下载结束（成功 / 失败 / 取消）追加到 `<home>/download_record.json`，与 CLI 共用同一份历史。
- API 统一返回 `{"success": bool, "data"?: ..., "error"?: ...}`；`GET /api/downloads` 读 SQLite 队列，不是历史文件。
- 暂停 / 恢复不在本实现支持范围内：`/api/pause-download`、`/api/resume-download` 返回 `success:false`。
- 可选 `--static <dir>` 托管前端导出（SPA 回退到 `index.html`）。
- `/api/health` 返回 `status/service/version`，供桌面壳探测。

完整端点与请求字段见 [`USAGE.md`](../USAGE.md#后台服务serve)。

## 7. 站点支持原则

- 支持范围 = 本机 yt-dlp 的 extractor 列表；用 `yt-dlp --list-extractors` 核对当前安装。
- YouTube：watch / youtu.be / shorts / playlist；片段裁剪需 FFmpeg；n-challenge 默认用 Node（`--js-runtimes`）。
- Bilibili：稿件、多 P、番剧、空间、收藏夹、合集、课堂、国际站、直播等；地址全表与限制见 [`docs/bilibili.md`](./bilibili.md)。
- X/Twitter：只支持单条 `/status/<id>`；公开推文默认走 syndication API；私密 / 受限内容需 cookie 并显式 `twitter:api=graphql`。
- 其他站点：同一 CLI；需要登录的站点传 cookie / proxy / extractor-args。

## 8. 验收标准

| 项 | 命令 |
|----|------|
| 可构建 | `go build -o vYtDL .`（Go 1.26+） |
| 单测通过 | `go test ./...` |
| 命令面完整 | `./vYtDL --help`、`./vYtDL download --help`、`./vYtDL records --help`、`./vYtDL analyze --help`、`./vYtDL serve --help` |
| 下载冒烟 | `./vYtDL download --no-tui -o ./downloads "<任意 yt-dlp 支持的公开 URL>"` |
| 历史可查 | `./vYtDL records --json --limit 5` |
| 守护进程可启动 | `./vYtDL serve --port 0` 后 `curl <addr>/api/health` |

## 9. 变更流程

- 新增用户可见 flag：同时更新 `USAGE.md`（参数表 + 示例）和 `skills/vytdl-cli/SKILL.md`（若影响 Agent 调用）。
- 改下载历史字段 / 路径：同时更新 `skills/download-record/SKILL.md` 与本文第 5 节。
- 改 `serve` API：同时更新 `USAGE.md` 的端点表。
- 站点范围变化（如 B 站地址表）：更新 `docs/bilibili.md`，不要写进代码白名单。
- 文档语言：中文优先；CLI flag、字段名、代码标识符保持英文。
