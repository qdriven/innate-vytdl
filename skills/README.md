# Skills

本项目自带的 Agent Skill（遵循 Agent Skills 开放标准：每个 skill 一个目录，唯一必需文件是带 frontmatter 的 `SKILL.md`）。本目录是**规范源**，`~/.cursor/skills` 等安装位置里的是副本。

| Skill | 用途 | 使用时机 |
|-------|------|----------|
| [`vytdl-cli`](./vytdl-cli/) | 构建 / 运行 / 扩展 vYtDL CLI；`download` / `records` / `analyze` / `serve`；多站点下载与排障 | 任何涉及 vYtDL 或命令行下载视频、查询历史、起后端服务的任务 |
| [`download-record`](./download-record/) | 查询统一下载记录：是否下过、字幕路径、失败原因 | 用户要核对下载历史、`download_record`、字幕文件在哪 |
| [`x-video-dl`](./x-video-dl/) | 下载 X (Twitter) 单条推文视频（yt-dlp + 浏览器 cookie 直调脚本） | 用户给出 x.com / twitter.com 的 `status` 链接要求下载视频 |

## 安装到各 Agent 工具

```bash
REPO=$(git rev-parse --show-toplevel)   # 在本仓库根目录执行

# 复制安装（Cursor / Codex / Trae / WorkBuddy）
mkdir -p ~/.cursor/skills
cp -R "$REPO/skills/vytdl-cli" "$REPO/skills/download-record" "$REPO/skills/x-video-dl" ~/.cursor/skills/
# ~/.codex/skills、~/.trae/skills、~/.workbuddy/skills 同理

# 符号链接安装（跟随更新，zcode / Kimi Code）
mkdir -p ~/.zcode/skills ~/.kimi-code/skills
ln -sfn "$REPO/skills/vytdl-cli"      ~/.zcode/skills/vytdl-cli
ln -sfn "$REPO/skills/download-record" ~/.zcode/skills/download-record
ln -sfn "$REPO/skills/x-video-dl"      ~/.zcode/skills/x-video-dl
ln -sfn "$REPO/skills/vytdl-cli"      ~/.kimi-code/skills/vytdl-cli
```

各工具的具体安装路径、验证方法和任务示例见 [`docs/skills-guide.md`](../docs/skills-guide.md)。

## 相关文档

- [`USAGE.md`](../USAGE.md) — CLI 完整用法（参数、站点、serve API）
- [`docs/spec.md`](../docs/spec.md) — 用途与能力边界
- [`docs/use-cases.md`](../docs/use-cases.md) — 端到端用例（含 Agent Skill 调用）
- [`docs/bilibili.md`](../docs/bilibili.md) — B 站可下载范围

> 规范源是本目录；更新 skill 后，复制安装需重新 `cp -R`，符号链接安装自动生效。
