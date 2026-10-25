# Skills

本项目自带的 Agent Skill（遵循 [Agent Skills 开放标准](https://code.claude.com/docs/en/skills)：每个 skill 一个目录，唯一必需文件是带 frontmatter 的 `SKILL.md`）。

| Skill | 用途 | 使用时机 |
|-------|------|----------|
| [`vytdl-cli`](./vytdl-cli/) | 构建 / 运行 / 扩展 vYtDL CLI；多站点下载；与原生 yt-dlp 对比 | 任何涉及 vYtDL 或命令行下载视频的任务 |
| [`x-video-dl`](./x-video-dl/) | 下载 X (Twitter) 推文视频（yt-dlp + 浏览器 cookie 直调方案） | 用户给出 x.com / twitter.com 链接要求下载视频 |

## 安装到各 Agent 工具

```bash
REPO=/Users/patrick/innate/innate-works/projects/innate-vytdl

# 复制安装（Cursor / Codex / Trae / WorkBuddy）
mkdir -p ~/.cursor/skills && cp -R $REPO/skills/vytdl-cli ~/.cursor/skills/
# ~/.codex/skills、~/.trae/skills、~/.workbuddy/skills 同理

# 符号链接安装（跟随更新，zcode / Kimi Code）
ln -sfn $REPO/skills/x-video-dl ~/.zcode/skills/x-video-dl
ln -sfn $REPO/skills/vytdl-cli ~/.kimi-code/skills/vytdl-cli
```

各工具的具体安装路径、验证方法和任务示例见 [`docs/skills-guide.md`](../docs/skills-guide.md)。

> 规范源是本目录；各工具目录里的是安装副本。更新 skill 后需重新复制（或改用符号链接）。
