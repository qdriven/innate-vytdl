# 数据与状态契约（Data Contract）

本文件定义 vYtDL 的全部持久化数据：谁写、写在哪、什么格式、不要做什么。实现见 `internal/record`、`internal/playliststate`、`internal/server/database.go`。

> 文档与代码冲突时以代码为准，并应回改本文件。

## 1. 文件总表

| 文件 / 目录 | 所有者 | 内容 | 不要做什么 |
|-------------|--------|------|------------|
| `$VYTDL_HOME/download_record.json`（默认 `~/.vytdl/download_record.json`） | CLI + `serve` | 统一下载历史：`version: 1` + `records[]`，字段见第 2 节 | 不要另建 `subtitle_mapping.json` |
| `$VYTDL_HOME/download_record.csv` | CLI（`--log-format csv`） | 同一份历史的 CSV 形式，`subtitles` 列用 `\|` 分隔 | 不要把它当第二份历史 |
| `$VYTDL_HOME/vytdl.db` | 仅 `serve` | SQLite：`downloads`（实时队列）、`settings`、`vtt_reports` | 不要用它回答「历史上下过什么」 |
| `<输出目录>/<播放列表标题>/.playlist_state.json` | CLI | 播放列表逐条续传状态，见第 4 节 | 不要当下载历史 |
| `<输出目录>/<播放列表标题>/` | CLI | 该播放列表的成片与字幕 | — |
| `<home>/downloads/` | `serve` | serve 模式默认下载目录（`VYTDL_OUTPUT_DIR` / 设置可改） | — |

`$VYTDL_HOME` 未设置时取 `~/.vytdl`；home 目录都取不到时回退相对路径 `.vytdl`。

## 2. 历史记录字段（`internal/record/record.go`）

JSON 形态（`records --json` 的输出同构）：

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

- `subtitles` 是路径数组，没有字幕时为 `[]`（不是 `null`）。
- `duration` 是 Go duration 字符串（如 `1m2s`）。
- CSV 表头与上述 11 个字段一一对应；`subtitles` 用 `|` 连接。旧版没有 `subtitles` 列的 CSV 也能导入（按位置回退）。
- 同一 URL 允许多条记录（失败后再成功），用 `started_at` 区分；去重键包含 `started_at`，不会覆盖历史。
- 写入加锁（`<历史文件>.lock`，等待 10s，30s 以上陈旧锁会被清理）并临时文件 + 原子替换，CLI 与 `serve` 并发写不会写坏。
- 写入时机：CLI `download` 在整条命令结束后一次性追加；`serve` 在每次下载结束（成功 / 失败 / 取消）后立即追加。

## 3. `--record-file` 路径解析

| 输入 | 解析结果 |
|------|----------|
| 空 | `$VYTDL_HOME/download_record.json`（或 `--log-format csv` 时 `.csv`） |
| 裸文件名（不含 `/` 或 `\`） | 放在 `$VYTDL_HOME` 下 |
| 绝对路径或含分隔符的路径 | 按原路径使用 |
| 扩展名不是 `.json` / `.csv` | 自动追加对应扩展名（`foo.txt` → `foo.txt.json`） |

## 4. 播放列表续传状态（`internal/playliststate`）

路径：`<输出目录>/<清洗后的播放列表标题>/.playlist_state.json`（标题替换文件系统非法字符、截断 120 字符）。

```json
{
  "playlist_url": "...",
  "playlist_title": "...",
  "playlist_dir": "...",
  "updated_at": "...",
  "entries": [
    {
      "id": "...",
      "url": "...",
      "title": "...",
      "status": "succeeded",
      "error": "",
      "attempts": 1,
      "filename": "...",
      "subtitles": ["..."],
      "last_started_at": "...",
      "last_finished_at": "..."
    }
  ]
}
```

- `status` 取值：`pending` / `running` / `succeeded` / `failed`。
- `error`、`filename`、`subtitles`、`last_started_at`、`last_finished_at` 为空时省略（omitempty）。
- 播放列表条目 URL 使用 yt-dlp 返回的 `webpage_url`，不伪造地址。
- 续传语义见 [`cli.md` 第 3.6 节](./cli.md#36-播放列表模式-p)；`.playlist_state.json` 不是下载历史。

## 5. `vytdl.db`（仅 serve）

- 表：`downloads`（实时队列，含序列化参数列 `options`）、`settings`、`vtt_reports`。
- 启动时恢复上次处于 `pending` / `downloading` 的下载（`downloading` 先重置为 `pending` 再入队）。
- 查询历史用 `vYtDL records`，不要查这张库。完整行为见 [`serve.md`](./serve.md)。

## 6. 旧双文件记录导入

旧版用 `download_record.*` + `subtitle_mapping.*` 两份文件。当前实现只认**固定四个文件名**：`download_record.json`、`download_record.csv`、`subtitle_mapping.json`、`subtitle_mapping.csv`（不是按通配符扫描）。

- `download`：检查**输出目录 + 当前目录**；`records`：只检查当前目录。
- 命中即并入统一历史（按去重键合并），然后删除旧文件；若旧 record 文件本身就是当前活动历史文件则不删。
- 导入成功后打印 `Imported N legacy record(s) into <path>`。
- 不要再创建或更新 `subtitle_mapping.json`；字幕路径只在历史记录的 `subtitles` 字段里。
