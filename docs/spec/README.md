# vYtDL Spec（规范）

本目录是 vYtDL 的**规范集**：说明这个项目做什么、不做什么、命令行与数据的契约。它与 `README.md`（入口）、`USAGE.md`（完整用法）、`docs/use-cases.md`（场景用例）构成一套文档。

> 本仓库是独立 Go 项目（module `github.com/innate/yt-dl`），不依赖任何其他仓库。
> 文档与代码冲突时以代码为准，并应回改本目录。

## 规范索引

| 文件 | 内容 |
|------|------|
| [`cli.md`](./cli.md) | **CLI 规范**：命令面、`download` / `records` / `analyze` 参数契约、默认行为、输出与退出码 |
| [`data.md`](./data.md) | 数据与状态契约：统一历史、`vytdl.db`、`.playlist_state.json`、旧文件导入 |
| [`serve.md`](./serve.md) | `serve` 守护进程契约：启动参数、队列语义、API 响应格式、WS 事件 |

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

## 4. 站点支持原则

- 支持范围 = 本机 yt-dlp 的 extractor 列表；用 `yt-dlp --list-extractors` 核对当前安装。
- YouTube：watch / youtu.be / shorts / playlist；片段裁剪需 FFmpeg；n-challenge 默认用 Node（`--js-runtimes`）。
- Bilibili：稿件、多 P、番剧、空间、收藏夹、合集、课堂、国际站、直播等；地址全表与限制见 [`docs/bilibili.md`](../bilibili.md)。
- X/Twitter：只支持单条 `/status/<id>`；公开推文默认走 syndication API；私密 / 受限内容需 cookie 并显式 `twitter:api=graphql`。域名表见 [`cli.md` 第 3.8 节](./cli.md#38-x--twitter)。
- 其他站点：同一 CLI；需要登录的站点传 cookie / proxy / extractor-args。

## 5. 验收标准

| 项 | 命令 |
|----|------|
| 可构建 | `go build -o vYtDL .`（Go 1.26+） |
| 单测通过 | `go test ./...` |
| 命令面完整 | `./vYtDL --help`、`./vYtDL download --help`、`./vYtDL records --help`、`./vYtDL analyze --help`、`./vYtDL serve --help` |
| 下载冒烟 | `./vYtDL download --no-tui -o ./downloads "<任意 yt-dlp 支持的公开 URL>"` |
| 历史可查 | `./vYtDL records --json --limit 5` |
| 守护进程可启动 | `./vYtDL serve --port 0` 后 `curl <addr>/api/health` |

## 6. 变更流程

- 新增用户可见 flag：同时更新 `USAGE.md`（参数表 + 示例）、[`cli.md`](./cli.md)（参数契约）和 `skills/vytdl-cli/SKILL.md`（若影响 Agent 调用）。
- 改下载历史字段 / 路径：同时更新 [`data.md`](./data.md)、`skills/download-record/SKILL.md` 与 `USAGE.md`。
- 改 `serve` API：同时更新 [`serve.md`](./serve.md)（契约级）与 `USAGE.md` 的端点表；保持响应 envelope 与前端字段名。
- 站点范围变化（如 B 站地址表）：更新 `docs/bilibili.md`，不要写进代码白名单。
- 文档语言：中文优先；CLI flag、字段名、代码标识符保持英文。
