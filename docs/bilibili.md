# B 站：当前 skill 能下载什么

结论：**能下。** 用 [`skills/vytdl-cli`](../skills/vytdl-cli/SKILL.md)。vYtDL 没有单独的 B 站客户端，也没有域名白名单；`download` 把 URL 原样交给本机 **yt-dlp**。

另外两个 skill 不负责下载 B 站：

| Skill | 和 B 站的关系 |
|-------|----------------|
| `vytdl-cli` | 下载 B 站，以及 yt-dlp 支持的其他站点 |
| `download-record` | 下载完成后查 `~/.vytdl/download_record.json` |
| `x-video-dl` | 只处理 X/Twitter |

核对环境：本机 `yt-dlp` **2026.08.19**。对公开稿件做了元数据探测（不下完整文件）：

```text
yt-dlp --no-playlist --skip-download --print "%(extractor)s | %(id)s | %(title)s | %(ext)s" \
  "https://www.bilibili.com/video/BV13x41117TL"
→ BiliBili | BV13x41117TL | 阿滴英文｜英文歌分享#6 "Closer | mp4
```

extractor 名单来自这次安装的 `yt-dlp --list-extractors` 和 `yt_dlp/extractor/bilibili.py` 的 `_VALID_URL`。换了 yt-dlp 版本后，以 `yt-dlp --list-extractors` 里的 `Bili*` 为准。

## 调用

```bash
# 当前这一 P（默认 --no-playlist）
./vYtDL download --no-tui -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"

# 多分 P、番剧季、空间投稿、收藏夹、合集：加 --playlist，可断点续传
./vYtDL download --no-tui --playlist -o ./downloads -q 1080 \
  --cookies-from-browser chrome \
  "https://www.bilibili.com/video/BVxxxxxx"
```

指定某一 P：URL 加 `?p=2`，并且不要加 `--playlist`。续传状态在 `./downloads/<标题>/.playlist_state.json`。下完用 `./vYtDL records --json --limit 5` 看文件和字幕路径。

## 能下载的地址

| 内容 | 地址 | 命令 |
|------|------|------|
| 普通稿件（BV / av），含多分 P | `https://www.bilibili.com/video/BVxxxx`、`.../video/av123`、`.../festival/...?bvid=BV...` | 单 P 直接下；全部分 P 加 `--playlist` |
| 番剧单集 | `https://www.bilibili.com/bangumi/play/ep123` | 直接下 |
| 番剧整季 | `https://www.bilibili.com/bangumi/play/ss123` | `--playlist` |
| 番剧资料页 | `https://www.bilibili.com/bangumi/media/md123` | `--playlist` |
| 用户投稿 | `https://space.bilibili.com/<uid>`、`.../<uid>/video` | `--playlist` |
| 用户音频列表 | `https://space.bilibili.com/<uid>/audio` | `--playlist` |
| 合集 | `https://space.bilibili.com/<uid>/lists/<sid>`、`.../channel/collectiondetail?sid=<sid>` | `--playlist` |
| 系列（含直播回放列表） | `https://space.bilibili.com/<uid>/lists/<sid>?type=series`、`.../channel/seriesdetail?sid=<sid>` | `--playlist` |
| 收藏夹 | `https://space.bilibili.com/<uid>/favlist?fid=<fid>`、`https://www.bilibili.com/medialist/detail/ml<fid>` | `--playlist` |
| 播放列表页 | `https://www.bilibili.com/list/<id>`、`https://www.bilibili.com/medialist/play/<id>` | `--playlist` |
| 稍后再看 | `https://www.bilibili.com/watchlater` | `--playlist`，必须是本人登录 cookie |
| 音频 / 音频专辑 | `https://www.bilibili.com/audio/au123`、`.../audio/am123` | 单首直接下；专辑加 `--playlist` |
| 动态里的视频 | `https://t.bilibili.com/<id>`、`https://www.bilibili.com/opus/<id>` | 直接下，解析到对应稿件 |
| 课堂单集 / 整季 | `https://www.bilibili.com/cheese/play/ep123`、`.../cheese/play/ss123` | 整季加 `--playlist`；未购买的课时下不了 |
| 分区页 | `https://www.bilibili.com/v/<分区>/<子分区>`，如 `/v/kichiku/mad` | `--playlist`。这是分区列表，条目很多 |
| 搜索 | `bilisearch5:关键词`（前缀后的数字是条数） | 当作 URL 参数传入，并加 `--playlist` |
| 国际站 | `https://www.bilibili.tv/.../play/<season>/<ep>`、`.../video/<aid>`、`biliintl.com` 同类路径 | 单集直接下；系列页加 `--playlist` |
| 正在进行的直播 | `https://live.bilibili.com/<房间号>` | 只抓当前直播流（extractor 示例容器是 flv）。房间未开播就没有成片 |

## 登录、画质、字幕

| 情况 | 处理 |
|------|------|
| 1080p / 更高画质、CC 字幕 | `--cookies-from-browser chrome`（浏览器里已登录 bilibili.com）或 `--cookies cookies.txt`。未登录时 CC 字幕经常缺失，更高清晰度会提示需要大会员 |
| 大会员专享正片、付费课堂 | 登录并且账号本身有权限。课堂未购买时 yt-dlp 会要求登录并说明需要购买 |
| 别人的私密收藏夹、稍后再看 | 必须是该列表所有者的 cookie |
| 地区限制（「您所在的地区无法观看本片」或删除/地区错误） | `--proxy`，或换网络后再下 |
| 访问过频 | yt-dlp 会报 rate limit。隔一段时间再试，不要把 `-j` 开得很大 |

`-q 1080` 只是高度上限。实际上限取决于这条稿件和当前 cookie。

## 短链

`https://b23.tv/...` 在 yt-dlp 2026.08.19 里**没有**专用 extractor。把短链在浏览器里打开，改用跳转后的 `bilibili.com` 地址再交给 `vYtDL`。
