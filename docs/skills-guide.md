# Skills 指南：配合 CLI 在 WorkBuddy / zcode / Kimi 中使用

本项目的 Agent Skill 把「vYtDL CLI 怎么用」的知识（构建、参数、站点差异、排障）打包成 AI Agent 可自动加载的说明文件。装好 skill 后，你在 WorkBuddy / zcode / Kimi 里用自然语言说「下载这个视频」，Agent 就会按 skill 里的方法正确地调 CLI，不需要你记任何参数。

## Skill 是什么

遵循 Anthropic 2025-10 推出、2025-12 开放标准化的 **Agent Skills** 规范：每个 skill 是一个目录，唯一必需文件是带 frontmatter 的 `SKILL.md`：

```markdown
---
name: vytdl-cli
description: 构建、运行和扩展 vYtDL CLI……
---

（Agent 激活 skill 后才读到的完整说明）
```

关键机制是**渐进式披露**（Progressive Disclosure）：

1. Agent 启动时只看每个 skill 的 `name` + `description`（几十 token）；
2. 用户任务匹配 description 时才加载完整 `SKILL.md`；
3. 执行中按需读 `references/`、跑 `scripts/`。

所以 skill 解决的问题是：**给 Agent 很多知识和工作方法，又不把所有东西永远塞在 prompt 里。**

## 本项目自带的 Skill

| Skill | 作用 | 典型触发语 |
|-------|------|-----------|
| [`skills/vytdl-cli`](../skills/vytdl-cli/) | vYtDL CLI 的构建、使用、排障、扩展 | 「下载这个 YouTube/B站 视频」「播放列表断了续传」「vYtDL 报 403」 |
| [`skills/x-video-dl`](../skills/x-video-dl/) | X/Twitter 推文视频下载（yt-dlp + 浏览器 cookie 直调脚本） | 「把这个推文视频存下来」+ x.com 链接 |

两个 skill 覆盖 X/Twitter 场景时二选一即可：`x-video-dl` 更轻（无需构建 Go），`vytdl-cli` 更全（下载记录、播放列表、全站点）。

---

## 安装

规范源是本仓库 `skills/` 目录。两种安装方式：

```bash
REPO=/Users/patrick/innate/innate-works/projects/innate-vytdl

# 方式 A：符号链接（推荐）— 更新规范源后自动生效
ln -sfn $REPO/skills/vytdl-cli  ~/.zcode/skills/vytdl-cli
ln -sfn $REPO/skills/x-video-dl ~/.zcode/skills/x-video-dl

# 方式 B：复制 — 适合不支持 symlink 或要打包上云的工具
cp -R $REPO/skills/vytdl-cli ~/.workbuddy/skills/
```

更新 skill 后：链接安装自动生效；复制安装需重新 `cp -R`。

---

## WorkBuddy

**安装路径**：`~/.workbuddy/skills/<skill-name>/`（复制安装，WorkBuddy 启动时扫描该目录）；也可以在 WorkBuddy 界面「专家·技能·连接器 → 上传技能」导入 skill 目录的 zip 包，或从技能市场安装。

```bash
REPO=/Users/patrick/innate/innate-works/projects/innate-vytdl
mkdir -p ~/.workbuddy/skills
cp -R $REPO/skills/vytdl-cli ~/.workbuddy/skills/
cp -R $REPO/skills/x-video-dl ~/.workbuddy/skills/
```

**使用**：

1. 重启 WorkBuddy（或在技能面板确认 skill 已启用）；
2. 直接用自然语言下任务，Agent 依据 description 自动匹配；也可以输入 `/` 从技能列表显式唤起；
3. 任务示例：
   - 「用 vYtDL 下载这个视频到 ~/Downloads：https://www.bilibili.com/video/BVxxxxxx」
   - 「把这个播放列表全部下载，1080p，断点续传：<playlist URL>」
   - 「下载这条推文的视频：https://x.com/xxx/status/123」→ 匹配 `x-video-dl`

**注意事项**（来自实际使用教训，已写进 skill）：

- WorkBuddy 会给会话注入代理（如 `HTTP(S)_PROXY=127.0.0.1:52711`），可能报 `Tunnel connection failed: 502`。用 `scutil --proxy` 查系统代理后覆盖环境变量再跑 CLI：`HTTPS_PROXY=http://127.0.0.1:4780 ./vYtDL download ...`；先 `curl -m 10 -x <proxy> https://www.youtube.com` 验证。
- WorkBuddy 沙箱里没有 TTY 时 vYtDL 会自动回退纯文本进度，无需干预。

## zcode

**安装路径**：`~/.zcode/skills/<skill-name>/`，符号链接或复制均可。

```bash
REPO=/Users/patrick/innate/innate-works/projects/innate-vytdl
ln -sfn $REPO/skills/vytdl-cli  ~/.zcode/skills/vytdl-cli
ln -sfn $REPO/skills/x-video-dl ~/.zcode/skills/x-video-dl
```

**使用**：新会话中直接说任务，例如「下载这条推文的视频：<链接>」。zcode 启动时读取各 skill 的 frontmatter description，任务匹配后加载 SKILL.md 并执行其中的脚本 / CLI 命令。

**任务示例**：

- 「用 x-video-dl 存这个推文视频：<x.com 链接>」→ 运行 `skills/x-video-dl/scripts/x_dl.sh`，下载后用 ffprobe 报告分辨率 / 时长；
- 「把这个 B 站合集按 720p 下载并支持续传」→ 运行 `vYtDL download --no-tui --playlist -q 720 -o ./downloads <URL>`。

## Kimi Code

**安装路径**：`~/.kimi-code/skills/<skill-name>/`（符号链接或复制）。

```bash
REPO=/Users/patrick/innate/innate-works/projects/innate-vytdl
ln -sfn $REPO/skills/vytdl-cli  ~/.kimi-code/skills/vytdl-cli
ln -sfn $REPO/skills/x-video-dl ~/.kimi-code/skills/x-video-dl
```

**使用**：同 zcode —— Kimi Code 按标准 Agent Skills 规范加载，任务匹配 description 后读取 SKILL.md 执行。注意 Kimi Code 的 shell 环境：首次读取浏览器 cookie（`--cookies-from-browser`）可能弹 macOS 钥匙串授权，选「始终允许」。

## 其他工具（Cursor / Codex / Trae）

同样的目录约定：`~/.cursor/skills`、`~/.codex/skills`、`~/.trae/skills`，复制或符号链接安装。项目仓库内工作时，也可以放在工作区 `.agents/skills/`、`.claude/skills/`、`.cursor/skills/` 下随仓库共享。

---

## Skill + CLI 的协作原理

```
你：「下载这条推文的视频」+ 链接
        │  description 匹配
        ▼
Agent 加载 skills/x-video-dl/SKILL.md（或 vytdl-cli）
        │  按 skill 说明执行
        ▼
x_dl.sh <链接>   或   vYtDL download --no-tui -o ./downloads <链接>
        │
        ▼
yt-dlp 子进程（cookie 鉴权 / syndication API）
        │
        ▼
Agent 用 ffprobe / download_record.json 验证并汇报结果
```

skill 决定「怎么做」（流程、参数、排障），CLI 决定「做什么」（实际下载）。没有 skill 时你也可以直接跑 CLI —— 全部用法见 [`USAGE.md`](../USAGE.md)。

## 排障

| 症状 | 处理 |
|------|------|
| Agent 说没找到 skill | 确认安装目录名 = skill 目录名（含 `SKILL.md`）；链接是否失效（`ls -L`）；重启会话 |
| Agent 不触发 skill | 任务描述里带上触发词（视频下载、vYtDL、推文视频…），或 `/` 显式唤起 |
| 钥匙串弹窗 | macOS 首次读浏览器 cookie 需允许；选「始终允许」 |
| 代理 502 | 见 WorkBuddy 一节 |
| 复制安装的 skill 行为旧 | 重新 `cp -R` 规范源 |
