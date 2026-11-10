---
name: x-video-dl
description: "下载 X (Twitter) 单条推文中的视频：yt-dlp + 浏览器 cookie，选最高画质并合并音轨。用户给出 x.com / twitter.com / t.co 的 status 链接，或只给 handle、主页、/media、要求下载某账号近几个月的全部视频时使用。只给账号名不能枚举时间线。"
---

# X Video Download

一键下载 X (Twitter) 推文视频，自带脚本 `scripts/x_dl.sh`，默认保存到 `~/Downloads`。

## 目录结构

```
x-video-dl/
├── SKILL.md          # 本说明
└── scripts/
    └── x_dl.sh       # 下载脚本（封装 yt-dlp + cookie 鉴权）
```

## 前置条件

- `yt-dlp`、`ffmpeg`：`brew install yt-dlp ffmpeg`
- Chrome 已登录 x.com。本脚本默认用 `--cookies-from-browser` 走需要登录的 GraphQL 接口，以便同时覆盖受保护 / 敏感内容；vYtDL 对公开推文可自动改用 syndication API。
- macOS 首次读取 Chrome cookie 可能弹钥匙串授权，选「始终允许」

## 用法

```bash
scripts/x_dl.sh [-b browser] [-o 目录] <推文链接> [链接...]
```

示例：

```bash
scripts/x_dl.sh "https://x.com/IndianaFever/status/2099166214639513805"
```

- 默认下载到 `~/Downloads`，文件名格式 `作者-推文ID.mp4`（如 `Indiana Fever-2098874191521202176.mp4`）
- `-b firefox|safari|edge|chromium`：换 cookie 来源浏览器（哪个登录了 x.com 用哪个）
- `-o <dir>`：换下载目录
- 已存在的同名文件自动跳过（`--no-overwrites`）
- 链接末尾混入的杂字符（如多粘了一个字母 `...3805m`、中文句号）yt-dlp 能自行容忍/脚本会清理常见包裹符

## 只给 handle 时

不能按账号批量下载，也不能筛「近 3 个月」。本脚本和 vYtDL 都只接受**单条推文**。

本机 yt-dlp 2026.08.19 的 X extractor 只有 `twitter`、`twitter:card`、`twitter:amplify`、`twitter:broadcast`、`twitter:spaces`、`twitter:shortener`。`twitter` 要求地址里有 `/status/<id>` 或 `/statuses/<id>`。下面这些地址对不上，不要传给脚本：

- `https://x.com/<handle>`
- `https://x.com/<handle>/media`
- 只有一个 @handle、没有 status id

原因：

- syndication（vYtDL 对公开推文的默认接口）按一条推文取视频，没有用户时间线接口。
- `--playlist` 不会去扫该账号的发帖。主页导不出条目时，vYtDL 会把这个 URL 当成一条去下。
- CLI 没有 `--dateafter`，近 3 个月无法在这里过滤。

用户已经有多条 status 链接时，可以逐条传入本脚本，或一次传给 `vYtDL download`。一条推文里有多段视频时，yt-dlp 把它看成小播放列表；用 vYtDL 要给**这一条 status** 加 `--playlist`，默认 `--no-playlist` 只下其中一段。

保护账号、年龄限制、仅登录可见的推文，syndication 拿不到。这时用本脚本（浏览器 cookie），或：

```bash
./vYtDL download --no-tui -o ./downloads \
  --cookies-from-browser chrome \
  --extractor-args "twitter:api=graphql" \
  "https://x.com/user/status/123"
```

显式 `twitter:api=` 会关掉 syndication 回退。

## Agent 工作流

1. 只有 handle、主页或「近 N 个月全部视频」时：说明做不到，请用户给出 `status` 链接。不要把主页 URL 交给脚本或 `--playlist`。
2. 从用户消息中取出推文链接（x.com / twitter.com / t.co 的 status 或短链），直接传给脚本。多条就多传几个参数。
3. 运行脚本下载，等待完成
4. 用 ffprobe 验证并报告：文件路径、分辨率、时长、大小
5. 输出文件中的推文 ID 与用户链接中的不同属**正常**：用户给的常是引用推文，视频属于被引的原推，yt-dlp 会自动跟随

```bash
ffprobe -v quiet -show_entries format=duration,size \
  -show_entries stream=codec_type,codec_name,width,height \
  -of default=noprint_wrappers=1 "<下载的文件>"
```

## 排查

- **Unsupported URL / 拿不到格式**：cookie 失效或浏览器未登录 x.com → 确认登录态，或 `-b` 换其他已登录浏览器
- **敏感内容 / 受限推文**：同样依赖登录态才能取到
- **进度卡住**：X 的 HLS 分片偶尔很慢，重跑即可，已完成分片不会重下
- **下载的是图片或纯文本**：该推文本来就没有视频，用 `yt-dlp --list-formats <链接>` 确认

## 与 vYtDL CLI 的关系

本 skill 与 vYtDL CLI（本仓库主程序）覆盖同一场景，二选一即可：

- **本 skill**：直调 yt-dlp + 浏览器 cookie，文件名固定 `作者-推文ID.mp4`，适合快速单条下载
- **vYtDL CLI**：`./vYtDL download --no-tui -o ./downloads "<推文链接>"` — 公开推文无需 cookie（自动走 syndication API），带下载记录；私密/受限推文仍需 `--cookies-from-browser chrome --extractor-args "twitter:api=graphql"`

脚本路径相对仓库根目录：`skills/x-video-dl/scripts/x_dl.sh`。详见 [`USAGE.md`](../../USAGE.md#x--twitter)、[`docs/use-cases.md`](../../docs/use-cases.md#uc-4xtwitter-单条推文)、[`docs/spec.md`](../../docs/spec.md)。

