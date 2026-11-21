# serve 契约（后台服务规范）

本文件定义 `vYtDL serve` 守护进程的行为契约：启动参数、网络边界、队列语义、API 响应格式。实现见 `cmd/serve.go` 与 `internal/server`。

> 文档与代码冲突时以代码为准，并应回改本文件。完整端点表、请求字段、WebSocket 事件与设置键见 [`USAGE.md` 的 serve 一节](../../USAGE.md#后台服务serve)。

## 1. 定位

`serve` 是 Web / 桌面前端的**唯一后端**：HTTP API + WebSocket + 下载队列 + SQLite 持久化 + VTT 分析 + 音频提取，可选静态托管前端导出。本仓库不实现前端 UI。

## 2. 启动参数与环境变量

| 参数 | 默认 | 环境变量 | 含义 |
|------|------|----------|------|
| `--port` | `0` | `PORT` | 监听端口；0 = 随机空闲端口（桌面壳就绪探测用） |
| `--host` | `127.0.0.1` | `VYTDL_HOST` | 绑定地址；Docker 用 `0.0.0.0` |
| `--static` | 空 | `VYTDL_STATIC_DIR` | 托管前端导出目录（含 SPA 回退 `index.html`）；目录不存在时启动失败 |
| `--home` | `~/.vytdl` | `VYTDL_HOME` | 数据根：`vytdl.db`、下载历史 |
| `--dev` | 关 | — | 请求日志 |

- 默认下载目录解析链：`VYTDL_OUTPUT_DIR` 环境变量 → `config.json` 的 `output_dir` → `<home>/downloads`。前端设置 `default_output_dir` 仍作用于入队请求。
- 启动横幅（stdout）：监听地址、data root、downloads 目录、static 目录（配置时）。
- 收到 SIGINT / SIGTERM：取消进行中的下载，优雅退出。
- CORS 允许所有来源；默认只绑回环，Docker 部署时自行控制网络暴露。
- 静态托管只接受 GET / HEAD；未命中的路径回退 `index.html`。
- `GET /api/health` 返回 `status` / `service` / `version`，供桌面壳探测。

## 3. API 响应格式

统一 envelope：

```json
{"success": true, "data": ...}
{"success": false, "error": "..."}
```

暂停 / 恢复不在实现范围内：`POST /api/pause-download`、`POST /api/resume-download` 返回 HTTP 200 + `{"success": false, "error": "Pause/resume is not supported in web mode"}`。

## 4. 队列语义

- 队列并发上限来自设置 `max_concurrent_downloads`（默认 3，范围 1–10），修改后立即生效。
- 启动时恢复上次处于 `pending` / `downloading` 的下载（参数存在 `downloads.options` 列；`downloading` 先重置为 `pending`）。
- **取消**：作用于排队中或进行中的下载；对已结束的记录（如已完成）调用也会把队列库里的状态改写为 `cancelled` 并广播事件。
- **重试**：以单条重新入队（不复用原播放列表模式），返回新 ID。
- **删除**：先取消再删记录。
- `GET /api/downloads` 读 SQLite 队列 `vytdl.db`，**不是**下载历史文件。
- 每次下载结束（成功 / 失败 / 取消）追加到 `<home>/download_record.json`，与 CLI 共用同一份历史（见 [`data.md`](./data.md)）。

## 5. WebSocket（`GET /api/ws`）

daemon 只发送、不要求客户端消息（入站消息被丢弃）。事件：

```json
{"event": "download:progress:<id>", "data": {...}}
{"event": "download:status:<id>", "data": "downloading|cancelled|completed|failed"}
{"event": "download:complete:<id>", "data": {"success": true, "data": {...}}}
{"event": "download:error:<id>", "data": {"success": false, "error": "..."}}
{"event": "download:log:<id>", "data": {"level": "...", "message": "..."}}
{"event": "queue:update", "data": {"id": "...", "status": "pending|cancelled|..."}}
{"type": "vtt-report:status", "reportId": "...", "status": "pending|processing|failed"}
{"type": "vtt-report:complete", "reportId": "..."}
```

注意：`download:status:<id>` 不会以 `pending` 值出现；入队的 `pending` 只通过 `queue:update` 广播。`download:complete` / `download:error` 的 data 是 envelope 包裹结构。

## 6. 服务模式与 CLI 下载的差异

`serve` 组装下载参数时额外强制（CLI 不带这些行为）：

- 默认 `--ignore-config`（除非请求带 `config_location`），用户级 yt-dlp 配置不生效；
- 强制 `--windows-filenames`；
- 自动字幕加 `--sleep-subtitles 1`（防限流）；
- 片段裁剪使用 muxed-safe 选择器并关闭 direct merge（防进度卡死）；
- X/Twitter 自动 `twitter:api=syndication`（同 CLI）；
- `serve` 还接受 CLI 没有的下载参数：`format_id`、`rate_limit`、`concurrent_fragments`、`embed_thumbnail` / `embed_metadata` / `embed_chapters`、`sponsorblock_remove`、`filename_template`、`po_token`、`config_location`、`cookie {mode, path, browser}`。

## 7. VTT 分析与音频提取

- `POST /api/analyze-vtt`：依次尝试字幕语言 `zh → en → ja → ko → zh-Hans → zh-Hant`，下载到 `<输出目录>/vtt-temp/<reportId>`，解析成 Markdown 存进 `vtt_reports`，完成后清理临时目录；进度经 WS 推送。
- `POST /api/extract-audio`：调 FFmpeg；`audio_format` 支持 `mp3`（默认）、`m4a`/`aac`、`flac`、`wav`、`ogg`、`opus`；返回 `data.audio_path`；FFmpeg 路径可用 `FFMPEG_PATH` 覆盖。
- `GET /api/vtt-reports`：分页 `page` / `limit`（默认 20，上限 100）/ `lang` 过滤。
- 打开 / 定位目录（`open-download-folder` / `reveal-in-folder`）在 Docker 里会失败，返回 `success:false`（预期行为，前端会提示）。
