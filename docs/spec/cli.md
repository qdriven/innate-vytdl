# CLI Spec（命令行规范）

本文件是 vYtDL CLI 的**命令行契约**：命令面、参数语义、默认值、输出与退出码。实现见 `cmd/` 与 `internal/downloader`。

- 面向人的用法与示例：[`USAGE.md`](../../USAGE.md)
- 数据与状态契约（历史文件、`vytdl.db`、续传状态）：[`data.md`](./data.md)
- `serve` 守护进程契约：[`serve.md`](./serve.md)
- 定位与能力边界：[`README.md`](./README.md)

> 文档与代码冲突时以代码为准，并应回改本文件。

## 1. 程序标识

| 项 | 值 |
|----|----|
| Go module | `github.com/innate/yt-dl`（独立仓库，Go 1.26+） |
| 构建产物 | 二进制名 `vYtDL`（`go build -o vYtDL .`） |
| Cobra 根命令 `Use` | `yt-dl` —— `--help` 里显示 `yt-dl`；与二进制名是同一个程序 |

根命令 `SilenceUsage: true`：出错时只打印错误，不重复 usage。没有 `--version` flag；版本号只出现在 `serve` 的启动输出与 `/api/health`（`1.0.0`）。

## 2. 命令面

| 命令 | 别名 | 位置参数 | 作用 |
|------|------|----------|------|
| `download [flags] <url> [url…]` | `dl`、`get` | ≥ 1 个 URL；`--install-yt-dlp` 或 `--batch-file` 时可省略 | 下载视频 / 播放列表 |
| `records` | — | 不接受（NoArgs） | 查询统一下载历史 |
| `analyze [flags] [<file.vtt>]` | `an`、`ana` | 0–1 个文件；缺省读 stdin | VTT → 纯文本 / Markdown |
| `serve` | — | 不接受 | 启动 HTTP API + WebSocket 守护进程（契约见 [`serve.md`](./serve.md)） |
| `completion <shell>` | — | — | Cobra 自动生成的 shell 补全 |

## 3. `download`

### 3.1 参数

| Flag | 短 | 默认 | 语义 |
|------|----|------|------|
| `--format` | `-f` | `mp4` | 合并容器，传 `--merge-output-format`；不做转码 |
| `--quality` | `-q` | 空 | 画质选择，见 3.2 |
| `--start` / `--end` | — | 空 | 片段裁剪起止点（`HH:MM:SS` 或秒），见 3.5 |
| `--output` | `-o` | `config.json` 的 `output_dir`，否则 `.` | 输出目录（自动创建），见 3.3 |
| `--sub-langs` | — | `en,zh` | 逗号分隔的字幕语言列表 |
| `--no-subs` | — | 关 | 关闭全部字幕（手动 + 自动） |
| `--no-auto-subs` | — | 关 | 只关闭自动字幕 |
| `--playlist` | `-p` | 关 | 播放列表模式，见 3.6 |
| `--reset-playlist-state` | — | 关 | 丢弃续传状态，播放列表从头开始 |
| `--concurrency` | `-j` | `1` | 多个 URL 参数之间的并发上限，见 3.7 |
| `--no-tui` | — | 关 | 纯文本进度，见 3.9 |
| `--log-format` | — | `json` | 历史格式：`json` 或 `csv`（非法值直接报错退出） |
| `--record-file` | — | 空 | 历史文件路径；解析规则见 [`data.md`](./data.md) |
| `--yt-dlp-bin` | — | `config.json` 的 `yt_dlp_bin` | yt-dlp / youtube-dl 路径 |
| `--proxy` | — | 空 | HTTP/HTTPS/SOCKS 代理，原样透传 |
| `--cookies` | — | 空 | Netscape cookie 文件，原样透传 |
| `--cookies-from-browser` | — | 空 | 浏览器 profile 选择器，原样透传 |
| `--user-agent` | — | 空 | 自定义 UA，原样透传 |
| `--extractor-args` | — | 空 | yt-dlp extractor 参数，原样透传 |
| `--js-runtimes` | — | `node` | 透传 `--js-runtimes`（YouTube n-challenge） |
| `--retries` | — | `10` | 同时设置 `--retries` 与 `--extractor-retries` |
| `--socket-timeout` | — | `30` | 网络超时（秒） |
| `--force-ipv4` | — | 关 | 透传 `--force-ipv4` |
| `--install-yt-dlp` | — | 关 | 下载 / 更新 yt-dlp 到 `./bin` 后退出，无需 URL |
| `--batch-file` | `-a` | 空 | 从文本文件读 URL：每行一个，`#` 注释，`-` = stdin；与位置参数合并，可与 `-j` 并发 |

`download` **不透传**也**不提供**：格式 ID 精选、限速、分片并发、嵌入元数据/封面/章节、SponsorBlock、文件名模板、PO token 等 —— 这些是 `serve` 模式的下载参数（见 [`serve.md`](./serve.md)）；CLI 侧统一用 `-q` 表达画质。

### 3.2 画质选择（`-q`）

实现：`internal/downloader.buildArgs()`。

| `-q` 输入 | 实际传给 yt-dlp 的 `-f` |
|-----------|--------------------------|
| 空、`best`、`bestvideo+bestaudio` | `bestvideo+bestaudio/best` |
| `720`、`1080`、`2160`、`1080p`（`^\d+p?$`） | `bestvideo[height<=N]+bestaudio/best[height<=N]` |
| 其他字符串 | 原样作为 `-f` 选择器 |

`-q N` 是**高度上限**，不是精确画质；实际上限取决于站点、登录态与会话。

### 3.3 容器与输出

- `-f <container>` 只传 `--merge-output-format <container>`，是合并容器偏好；某站点没有可合并进该容器的流时，yt-dlp 仍可能输出其他容器。没有编码转换。
- `-o <dir>`：输出模板固定为 `<dir>/%(title)s.%(ext)s`；目录不存在会自动创建（0755）。
- `--output` 默认值取 `config.json` 的 `output_dir`（加载时展开 `~/` 前缀；典型用途：挂载的私有云盘 / NAS 卷），未设置时为当前目录 `.`；显式 `-o` 优先。
- 播放列表模式下每条视频落在 `<dir>/<播放列表标题>/` 子目录（标题做文件系统安全清洗，最长 120 字符）。

### 3.4 字幕

默认请求 `--write-subs --write-auto-subs --sub-langs en,zh`：

- `--no-subs`：完全不传字幕参数。
- `--no-auto-subs`：保留 `--write-subs`，去掉 `--write-auto-subs`。
- `--sub-langs zh`：覆盖语言列表（逗号分隔，空白项忽略）。
- `--sub-langs` 只有在开启字幕时才传给 yt-dlp。
- 字幕文件落在输出目录；路径写进历史记录的 `subtitles` 字段（见 [`data.md`](./data.md)）。

### 3.5 片段裁剪

`--start` / `--end` 生成 `--download-sections "*START-END" --force-keyframes-at-cuts`：

- 只给 `--start`：终点为 `inf`；只给 `--end`：起点为 `0`。
- 时间格式 `HH:MM:SS` 或秒数，由 yt-dlp 解析。
- 需要 FFmpeg。

### 3.6 播放列表模式（`-p`）

流程：

1. `yt-dlp --dump-single-json --flat-playlist` 拉取条目列表。
2. 创建 `<输出目录>/<播放列表标题>/`。
3. 写 `.playlist_state.json`，逐条下载（内部串行）。
4. 状态为 `succeeded` 的条目直接跳过（TUI 显示 `skipped`）。
5. `failed` 条目下次重试；被中断的 `running` 条目在下次启动时转为 `failed`。
6. **重跑完全相同的命令即续传**（相同 `-o` + 相同 URL → 同一份状态文件）；`--reset-playlist-state` 丢弃状态。
7. 状态标记成功但成片已被删除的条目会重新排队。

不传 `-p` 时一律追加 `--no-playlist`（与 URL 形态无关），避免单视频链接里的 `list=…` 被当成整列表。状态文件字段见 [`data.md`](./data.md)。

### 3.7 多 URL 并发（`-j`）

- URL 来源：位置参数 + `--batch-file`（`-a`）清单文件（每行一个 URL，`#` 注释，`-` = stdin），两处合并后统一调度；只有 `--batch-file` 时可省略位置参数。
- `-j 1`（默认）串行；`> 1` 时用信号量控制同时在跑的 URL 数；`< 1` 按 1 处理。
- 每个 URL 可来自不同站点；`-p` 作用于所有 URL 参数。
- 播放列表内部条目始终逐条下载，`-j` 不会拆分单个播放列表。
- 单个 URL 失败不会中断其余 URL；所有结果在整条命令结束后统一写入历史。
- `-j` 越高越容易触发站点限流，建议 1–3。

### 3.8 X / Twitter

- 识别域名：`x.com`、`twitter.com`（含其任意子域）、`vxtwitter.com`、`fxtwitter.com`、`nitter.net`（后三者不识别子域；`www.` / `mobile.` 前缀忽略）。
- 对上述域名的 URL，若用户没有在任何 `--extractor-args` 里显式传 `twitter:api=`，自动追加 `--extractor-args twitter:api=syndication`（公开推文开箱即用）。
- 用户显式传任何 `twitter:api=` 时**用户优先**，不追加回退（私密 / 受限内容用 cookie + `twitter:api=graphql`）。
- 只支持单条 `/status/<id>` 链接；不支持按 handle / 主页 / `/media` 枚举时间线，也没有日期过滤参数。一条推文里有多段视频时，给这条 status 加 `-p`。

### 3.9 进度输出

- 默认启动 Bubble Tea TUI（备用屏），显示每条视频的进度条、速度、ETA、完成 / 失败 / 跳过状态。
- `--no-tui`：纯文本，格式 `[12.3%] 标题 速度 ETA`、`[done] 标题`、`[error] 标题: 原因`。
- 无 TTY（脚本 / cron / CI / Agent 沙箱）时 TUI 启动失败会**自动回退**纯文本（stderr 提示），进度通道不会阻塞。

### 3.10 历史写入与旧文件导入

- 所有结果在整条命令结束后一次性追加到统一历史（`--record-file` / `--log-format` 决定文件），写入带锁、原子替换；契约见 [`data.md`](./data.md)。
- 结束时打印 `Download log: <path>` 与 `Completed: X succeeded, Y failed.`
- **所有下载完成之后、新记录写入之前**，会把**输出目录与当前目录**里的旧双文件记录（`download_record.json|csv` + `subtitle_mapping.json|csv`）并入统一历史并删除旧文件，详见 [`data.md`](./data.md)。

### 3.11 输出流与退出码

| 流 | 内容 |
|----|------|
| stdout | 纯文本进度行、`Imported N legacy record(s) into <path>`、`Download log: <path>`、`Completed: X succeeded, Y failed.`、`No pending playlist items…`（播放列表全部跳过时） |
| stderr | `Using yt-dlp: <path>`、`Loaded N URL(s) from <file>`（`--batch-file`）、TUI 回退提示、导入 / 写历史的 warning |

退出码：`0` 成功；`1` 任何错误 —— 参数校验失败（无 URL、非法 `--log-format`）、yt-dlp 不可用、**任一**下载失败（其余 URL 仍会跑完）。播放列表所有条目均已完成后重跑：打印提示并返回 `0`。

### 3.12 参数 → yt-dlp 映射

| vYtDL 参数 | yt-dlp 等价（近似） |
|-----------|------------------------------|
| `-q 1080` | `-f "bestvideo[height<=1080]+bestaudio/best[height<=1080]"` |
| `-f mp4` | `--merge-output-format mp4` |
| `-o ./dir` | `-o "./dir/%(title)s.%(ext)s"` |
| （默认字幕） | `--write-subs --write-auto-subs --sub-langs en,zh` |
| `--start` / `--end` | `--download-sections "*START-END" --force-keyframes-at-cuts` |
| （默认单视频） | `--no-playlist` |
| `--retries N` | `--retries N --extractor-retries N` |
| `--cookies` / `--cookies-from-browser` / `--proxy` / `--user-agent` / `--extractor-args` / `--js-runtimes` / `--socket-timeout` / `--force-ipv4` | 相同 |
| （内部固定） | `--newline --progress --print-json` |

代码路径：`cmd/download.go` → `internal/downloader/options.go` → `buildArgs()`（`internal/downloader/downloader.go`）。

## 4. `records`

### 4.1 参数

| Flag | 默认 | 语义 |
|------|------|------|
| `--log-format` | `json` | `json` 或 `csv`（决定读写哪份历史文件） |
| `--record-file` | 空 | 历史路径；解析规则同 `download`，见 [`data.md`](./data.md) |
| `--failed` | 关 | 只显示 `success = false` 的记录 |
| `--json` | 关 | 以 JSON 输出到 stdout（即使底层历史是 CSV） |
| `--limit` | `0` | 只取按时间序的**最后** N 条（0 = 全部）；展示仍最新在前 |

### 4.2 输出

- 运行前把**当前目录**里的旧双文件记录并入统一历史（`download` 还会检查输出目录）。
- stderr：`Imported N legacy record(s)…`（仅当导入 > 0）、`Download log: <path>`（总是）。
- 表格（stdout，最新在前）：列 `STARTED`（`2006-01-02 15:04:05`）、`OK`（`yes`/`no`）、`VIDEO`、`TITLE`（制表符 / 换行替换为空格）、`FILE`（basename）、`SUBS`（字幕文件数）。
- `--json`：`{"version": 1, "records": [...]}`，2 空格缩进；无记录时 `records` 为 `[]`。字段定义见 [`data.md`](./data.md)。

## 5. `analyze`

| Flag | 短 | 默认 | 语义 |
|------|----|------|------|
| `--mode` | `-m` | `text` | `text`：纯文本；`markdown`：带时间戳的 Markdown；其他值报错退出 |
| `--output` | `-o` | stdout | 写入文件（创建 / 截断） |
| `--segment` | `-s` | 空 | **保留参数位，当前无任何效果** |

- 位置参数 0–1 个：给文件路径，或从 stdin 读。
- `text`：整段纯文本（句间空格连接，无时间戳）；`markdown`：`# Transcript` + 按小时分组的 `## HH:MM:SS` + 每句前带时间戳。
- 解析器支持 YouTube 自动字幕的 `<c>` 单词级标签，会去除重复的 10ms 快照 cue（`internal/vtt`，与 `serve` 的 VTT 分析共用）。

## 6. yt-dlp 二进制解析

顺序（`internal/ytdlpbin`）：

1. 显式路径：`--yt-dlp-bin`（其默认值来自 `config.json` 的 `yt_dlp_bin`；`config.json` 有值时优先于 `YT_DL_BIN`）
2. `YT_DL_BIN` 环境变量
3. PATH 上的 `yt-dlp` 或 `youtube-dl`
4. 构建期内嵌二进制（`-tags embed_ytdlp`；释放为缓存目录里的 `yt-dlp-embedded`）
5. **`./bin/yt-dlp[.exe]`**（当前目录下的托管副本）：不存在时**自动从 GitHub Releases 下载到这里**（`VYTDL_YTDLP_MIRROR` 可换镜像）；`--install-yt-dlp` 重新下载即更新它
6. 用户缓存目录 `vYtDL` 下的 `yt-dlp[.exe]`（旧版本的自动下载位置，只读兼容，不再往这里下载）

- `--install-yt-dlp`：只执行「下载 / 更新到 `./bin`」后退出。
- 解析失败时报错并提示三种解决方式（显式路径 / PATH / 内嵌构建），退出码 1。
- 解析成功时在 stderr 打印 `Using yt-dlp: <path>`。
- `config.json` 有两个字段：`yt_dlp_bin` 与 `output_dir`（默认输出目录，`~/` 前缀会展开；`serve` 也读它）；查找顺序：`VYTDL_CONFIG` → 当前目录 `config.json` → 可执行文件目录 `config.json`。

## 7. 环境变量（CLI 侧）

| 变量 | 作用 |
|------|------|
| `VYTDL_HOME` | 数据根目录（历史文件默认位置；默认 `~/.vytdl`） |
| `VYTDL_CONFIG` | `config.json` 路径 |
| `YT_DL_BIN` | yt-dlp 显式路径（优先级见第 6 节） |
| `VYTDL_YTDLP_MIRROR` | 自动下载 yt-dlp 的镜像 URL 前缀 |

`serve` 侧的环境变量（`PORT`、`VYTDL_HOST`、`VYTDL_STATIC_DIR`、`VYTDL_OUTPUT_DIR`、`FFMPEG_PATH`）见 [`serve.md`](./serve.md)。

## 8. 扩展 CLI 的标准动作

1. `cmd/download.go` `init()` 加 flag；
2. `internal/downloader.Options` 加字段；
3. 需要时接入 `buildArgs()`；
4. 同步 `USAGE.md` 参数表与本文第 3.1 节；影响 Agent 调用时同步 `skills/vytdl-cli/SKILL.md`；
5. 补测试（`go test ./...`）。
