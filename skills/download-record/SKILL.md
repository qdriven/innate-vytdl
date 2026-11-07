---
name: download-record
description: |
  查询 vYtDL 的统一下载记录。当需要确认某个视频是否下过、失败原因、字幕文件路径、
  下载耗时，或用户提到 download_record、subtitle_mapping、下载历史、下载日志时使用。
  触发词：下载记录、下载历史、download_record、subtitle_mapping、vYtDL records。
---

# vYtDL 下载记录

历史只有一份目录文件。字幕路径写在每条记录里，没有单独的 mapping 文件。

## 文件位置

| 内容 | 路径 |
|------|------|
| 下载历史（CLI 和 `serve` 共用） | `$VYTDL_HOME/download_record.json`，默认 `~/.vytdl/download_record.json` |
| 实时队列（仅 `serve` / Web UI） | `$VYTDL_HOME/vytdl.db` |
| 播放列表续传 | `<输出目录>/<标题>/.playlist_state.json` |

查「下过什么、字幕在哪、为什么失败」只读下载历史。不要读 `vytdl.db` 或 `.playlist_state.json` 来回答这类问题。

## 怎么查

在仓库根目录：

```bash
go build -o vYtDL .
./vYtDL records                 # 最新在前的表格
./vYtDL records --json          # 完整 JSON，含字幕路径
./vYtDL records --failed        # 只看失败
./vYtDL records --limit 20
./vYtDL records --record-file /path/to/download_record.json --json
```

路径打在 stderr：`Download log: ...`。JSON 在 stdout。

旧的 `download_record.json` + `subtitle_mapping.json`（或同名 `.csv`）如果还在当前目录，`records` 和 `download` 会并入上面的目录并删除这两份旧文件。

## 记录字段

`version` 为 `1`。`records[]` 每条：

- `video_id` / `title` / `url`
- `output_dir` / `filename`：视频文件
- `subtitles`：字幕路径数组（没有则为 `[]`）
- `success` / `error`
- `started_at` / `finished_at` / `duration`

同一 URL 可以有多条（失败重试再成功）。用 `started_at` 区分，不要只按 URL 去重后丢掉失败原因。

## 调用方约定

1. 下载结束后看命令打印的 `Download log:`，或再跑 `./vYtDL records --json --limit 5`。
2. 核对文件是否落地时，用记录里的 `filename` 和 `subtitles`，不要假设文件名等于标题。
3. 不要再创建或更新 `subtitle_mapping.json`。
4. 换机器或指定目录时用 `VYTDL_HOME` 或 `--record-file`。CSV 用 `--log-format csv`（同样只有一个文件，字幕列用 `|` 分隔）。
