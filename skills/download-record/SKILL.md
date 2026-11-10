---
name: download-record
description: |
  查询 vYtDL 的统一下载记录。当需要确认某个视频是否下过、失败原因、字幕文件路径、
  下载耗时，或用户提到 download_record、subtitle_mapping、下载历史、下载日志时使用。
  触发词：下载记录、下载历史、download_record、subtitle_mapping、vYtDL records。
---

# vYtDL 下载记录

历史只有一份目录文件。字幕路径写在每条记录里，没有单独的 mapping 文件。CLI 与 `serve` 共用这份历史。

## 文件位置

| 内容 | 路径 |
|------|------|
| 下载历史（CLI 和 `serve` 共用） | `$VYTDL_HOME/download_record.json`，默认 `~/.vytdl/download_record.json` |
| 下载历史（CSV 模式） | `$VYTDL_HOME/download_record.csv`（`--log-format csv`） |
| 实时队列（仅 `serve` / Web UI） | `$VYTDL_HOME/vytdl.db` |
| 播放列表续传 | `<输出目录>/<标题>/.playlist_state.json` |

查「下过什么、字幕在哪、为什么失败」只读下载历史。不要读 `vytdl.db` 或 `.playlist_state.json` 来回答这类问题。

## 怎么查

在仓库根目录：

```bash
go build -o vYtDL .
./vYtDL records                 # 最新在前的表格（STARTED / OK / VIDEO / TITLE / FILE / SUBS）
./vYtDL records --json          # 完整 JSON，含字幕路径
./vYtDL records --failed        # 只看失败
./vYtDL records --limit 20
./vYtDL records --record-file /path/to/download_record.json --json
./vYtDL records --log-format csv
```

路径打在 stderr：`Download log: ...`。JSON 在 stdout；`--json` 即使历史是 CSV 也会输出 JSON。

`--record-file` 的解析规则：空 = `$VYTDL_HOME/download_record.json`；只给文件名（不含 `/`）= 放在 `$VYTDL_HOME` 下；给绝对路径或含 `/` 的路径 = 按原路径使用；缺扩展名会自动补 `.json` / `.csv`。

旧的 `download_record.json` + `subtitle_mapping.json`（或同名 `.csv`）如果还在当前目录，`records` 和 `download` 会并入上面的历史并删除这两份旧文件。

## 记录字段

`version` 为 `1`。`records[]` 每条：

- `video_id` / `title` / `url`
- `output_dir` / `filename`：视频文件
- `subtitles`：字幕路径数组（没有则为 `[]`）
- `success` / `error`
- `started_at` / `finished_at` / `duration`

CSV 表头与字段一一对应；`subtitles` 列用 `|` 分隔。

同一 URL 可以有多条（失败重试再成功）。用 `started_at` 区分，不要只按 URL 去重后丢掉失败原因。

## 写入时机

- `vYtDL download`：整条命令的所有结果结束后一次性追加（多个 URL / 播放列表都在一起）。
- `vYtDL serve`：每次下载结束（成功 / 失败 / 取消）后立即追加。
- 写入加锁（`<历史文件>.lock`）并原子替换，CLI 与 `serve` 并发写不会写坏。

## 调用方约定

1. 下载结束后看命令打印的 `Download log:`，或再跑 `./vYtDL records --json --limit 5`。
2. 核对文件是否落地时，用记录里的 `filename` 和 `subtitles`，不要假设文件名等于标题。
3. 不要再创建或更新 `subtitle_mapping.json`。
4. 换机器或指定目录时用 `VYTDL_HOME` 或 `--record-file`。

字段契约见 [`docs/spec.md`](../../docs/spec.md#5-数据与状态契约)；场景示例见 [`docs/use-cases.md`](../../docs/use-cases.md#uc-7查询下载历史)。
