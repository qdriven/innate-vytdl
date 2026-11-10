# AGENTS.md — vYtDL CLI（innate-vytdl）

给 AI Agent 的仓库指南。项目是**独立 Go module**（`github.com/innate/yt-dl`），不依赖其他仓库。二进制名 `vYtDL`，Cobra 根命令 `Use` 为 `yt-dl`。

## 快速命令

```bash
go build -o vYtDL .                 # Go 1.26+
go test ./...
./vYtDL download --no-tui -o ./downloads -q 1080 "URL"
./vYtDL records --json --limit 5
./vYtDL serve --port 0
```

`Taskfile.yml` 里还有 `task build`、`task build:all`、`task test`、`task install`；它统一设置 `GOWORK=off`。如果依赖下载失败，可以先 `GOPROXY=https://goproxy.cn,direct go mod download`。

## 先读哪个文档

| 任务 | 先读 |
|------|------|
| 了解项目定位 / 边界 | `docs/spec.md` |
| 改 CLI 参数或行为 | `USAGE.md` 的 `download` 参数表 + `internal/downloader` |
| 端到端演示 / 写示例 | `docs/use-cases.md` |
| 改 `serve` API | `USAGE.md` 的 `serve` 一节 + `internal/server` |
| 写 / 改 Agent Skill | 对应 `skills/*/SKILL.md` + `docs/skills-guide.md` |
| B 站地址范围 | `docs/bilibili.md` |

文档中文优先；CLI flag、JSON 字段、代码标识符保持英文。

## 代码地图

```
main.go                    cmd.Execute()
cmd/root.go                根命令（Use: yt-dl）、子命令注册
cmd/download.go            flags → downloader.Options；并发 worker pool；历史写入
cmd/records.go             历史查询；旧记录导入
cmd/analyze.go             VTT → text / markdown
cmd/serve.go               daemon 启动参数；信号优雅退出
internal/downloader/       buildArgs()、yt-dlp 子进程、进度解析、播放列表续传、X syndication
internal/ytdlpbin/         yt-dlp 解析：显式 → PATH → 内嵌 → 缓存 → 下载
internal/tui/              Bubble Tea 进度（无 TTY 回退纯文本）
internal/record/           统一历史 Catalog v1（JSON/CSV、锁、原子写、旧文件导入）
internal/playliststate/    <output>/<title>/.playlist_state.json
internal/vtt/              WebVTT 解析（analyze + serve 共用）
internal/server/           serve：HTTP API、WS Hub、队列、SQLite、VTT 分析、音频提取
internal/config/           config.json（仅 yt_dlp_bin；VYTDL_CONFIG 可覆盖）
scripts/                   build / fetch-ytdlp / shell 封装
skills/                    Agent Skill 规范源
docs/                      spec、use-cases、bilibili、skills-guide
```

## 不变量（改代码前先确认）

1. **不重新实现下载器 / 鉴权**：cookie、proxy、user-agent、extractor-args 原样透传给 yt-dlp。
2. **没有域名白名单**：站点覆盖 = 当前 yt-dlp 的 extractor；B 站范围只写在文档里。
3. **下载历史只有一份**：`$VYTDL_HOME/download_record.json`（默认 `~/.vytdl/download_record.json`）。字幕路径是每条记录的 `subtitles` 字段；CLI 与 `serve` 都追加它。**不要新增 `subtitle_mapping.json` 之类的第二份映射。**
4. **`vytdl.db` 只是 `serve` 的实时队列**（`downloads` / `settings` / `vtt_reports`）；回答「历史上下过什么」用 `vYtDL records`。
5. **`.playlist_state.json` 只是播放列表续传状态**，位于 `<输出目录>/<标题>/`。
6. **默认值保持**：容器 mp4、字幕 en+zh（含自动字幕）、`--js-runtimes node`、`-j 1`、单视频默认 `--no-playlist`。
7. **serve 与前端兼容**：统一响应 `{"success","data"|"error"}`；`GET /api/downloads` 读队列库；暂停/恢复返回 `success:false`。
8. **X/Twitter**：公开推文自动 `twitter:api=syndication`，用户显式 `twitter:api=` 优先；只支持单条 `/status/<id>`。

## 改动的标准动作

- **加用户可见 flag**：`cmd/*.go init()` 加 flag → `internal/downloader.Options` 加字段 → `buildArgs()` 接线（若需要）→ 更新 `USAGE.md` 参数表 → 更新 `skills/vytdl-cli/SKILL.md`。补测试（`go test ./...`）。
- **改历史字段/路径**：`internal/record` → 同步 `docs/spec.md` 第 5 节、`skills/download-record/SKILL.md`、`USAGE.md`。
- **改 serve API**：`internal/server/handlers.go`（路由）→ 同步 `USAGE.md` 端点表；保持响应 envelope 与前端字段名。
- **改播放列表行为**：`internal/downloader.DownloadPlaylist` + `internal/playliststate`；注意「重跑同命令续传」和「成功但文件丢失要重排」。
- **删旧文件**：先 `grep` 引用（README / USAGE / AGENTS / skills / scripts）；完全过时且不反映代码的文档可直接删除，并清理引用。

## 验证清单

```bash
go build -o vYtDL .
go test ./...

./vYtDL --help
./vYtDL download --help
./vYtDL records --help
./vYtDL analyze --help
./vYtDL serve --help

# 下载冒烟（任选公开 URL）
./vYtDL download --no-tui -o ./downloads "URL"
./vYtDL records --json --limit 1

# serve 冒烟
./vYtDL serve --port 0 &
curl http://127.0.0.1:<port>/api/health
```

## 文档维护规则

- 代码是唯一事实来源；文档只描述**当前实现**，不写未来计划（计划放 issue）。
- 用户可见行为变化 → 必须同步 `README.md`（入口/默认值）、`USAGE.md`（完整用法）、`docs/spec.md`（契约）中的相关部分。
- 面向 Agent 的调用约定放 `skills/*/SKILL.md`，不要复制整份 USAGE。
- 安装路径示例一律用占位符（如 `$REPO` / `$(git rev-parse --show-toplevel)`），不要写某台机器的本地路径。
- 已删文档的引用必须一起清掉，避免死链。
