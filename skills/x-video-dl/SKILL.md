---
name: x-video-dl
description: "下载 X (Twitter / 推特) 推文中的视频到本地：yt-dlp + 浏览器 cookie 鉴权，自动选最高画质并合并音轨。当用户给出 x.com / twitter.com / t.co 链接并要求下载、保存、抓取视频时使用，也适用于「把这个推文视频存下来」「下载这个视频」并附带推文链接的场景。"
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
- Chrome 已登录 x.com。X 的视频走 GraphQL 接口，**匿名拿不到**，必须借浏览器 cookie 鉴权
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

## Agent 工作流

1. 从用户消息中取出推文链接（x.com / twitter.com / t.co 均可），直接传给脚本
2. 运行脚本下载，等待完成
3. 用 ffprobe 验证并报告：文件路径、分辨率、时长、大小
4. 输出文件中的推文 ID 与用户链接中的不同属**正常**：用户给的常是引用推文，视频属于被引的原推，yt-dlp 会自动跟随

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
- **vYtDL CLI**：`vYtDL download --no-tui -o ./downloads "<推文链接>"` — 公开推文无需 cookie（自动走 syndication API），带下载记录；私密/受限推文仍需 `--cookies-from-browser chrome --extractor-args "twitter:api=graphql"`

详见仓库根目录 `USAGE.md` 的 X/Twitter 一节。

