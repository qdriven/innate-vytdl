#!/usr/bin/env bash
# x_dl.sh — 下载 X (Twitter) 推文视频到本地
# 依赖: yt-dlp + ffmpeg; 鉴权: 读取浏览器 cookie (默认 Chrome, 需已登录 x.com)
set -euo pipefail

BROWSER="chrome"
OUTDIR="$HOME/Downloads"

usage() {
  cat <<'EOF'
用法: x_dl.sh [-b browser] [-o 目录] <推文链接>...

选项:
  -b  cookie 来源浏览器: chrome(默认) | firefox | safari | edge | chromium
  -o  下载目录 (默认 ~/Downloads)

示例:
  x_dl.sh "https://x.com/IndianaFever/status/2099166214639513805"
EOF
  exit 1
}

while getopts ":b:o:h" opt; do
  case "$opt" in
    b) BROWSER="$OPTARG" ;;
    o) OUTDIR="$OPTARG" ;;
    *) usage ;;
  esac
done
shift $((OPTIND - 1))
[ $# -ge 1 ] || usage

for cmd in yt-dlp ffmpeg; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "错误: 缺少 $cmd, 请先 brew install $cmd" >&2; exit 1; }
done

mkdir -p "$OUTDIR"

# 清理链接首尾常见杂质: 空白、引号、括号、中英文句读
clean_url() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  s="${s#\"}"; s="${s%\"}"
  s="${s#\'}"; s="${s%\'}"
  s="${s#\(}"; s="${s%\)}"
  s="${s%[.。,，;；]}"
  printf '%s' "$s"
}

rc=0
for raw in "$@"; do
  url="$(clean_url "$raw")"
  echo "==> 下载: $url"
  yt-dlp \
    --cookies-from-browser "$BROWSER" \
    --no-overwrites \
    -o "$OUTDIR/%(uploader)s-%(id)s.%(ext)s" \
    "$url" || rc=1
done

echo "==> 完成, 下载目录: $OUTDIR (文件名格式: 作者-推文ID.mp4)"
exit $rc
