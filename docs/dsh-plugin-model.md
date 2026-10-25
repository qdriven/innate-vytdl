# DSH 插件模式如何落地：第一个完整项目（vYtDL）

本文说明 **DSH Desktop 的插件架构如何被借用、改造，并在 vYtDL 上完成了第一个端到端落地**——即「宿主 + 插件」前端架构的第一次完整实践。

> 权威设计文档与实现代码在 vYtDL 产品线所在的 monorepo（innate-keepthem）：
> 设计 `docs/refactor/sprint-0/02-plugin-model.md` · 协议 `docs/plugin-protocol.md` · 实现 `packages/plugin-runtime`。

## DSH 是什么

[DSH Desktop](https://github.com/anywhere-labs/deepseek-harness-desktop)（源码工作区 `~/innate/app-factory/projects/dsh-desktop`）是基于 DeepSeek Harness 构建的开源桌面客户端，核心理念是「**万物皆插件**，桌面本身也是插件」：

- 功能以插件形式挂载，桌面窗口只是宿主骨架；
- 插件带 manifest 契约、权限白名单、版本化 engine 字段；
- 宿主对插件故障做**错误隔离**：单个插件崩溃不拖垮整页；
- 有 market-installer 负责第三方插件的安装与校验。

## vYtDL 借用了 DSH 的什么

vYtDL 桌面版重构（Refactor Sprint 0-4，2026-10）把这套模式移植为前端架构。逐项对应：

| DSH 概念 | vYtDL 落地 | 位置（innate-keepthem） |
|----------|-----------|------------------------|
| 插件 manifest 契约 | `plugin.json`：`id`（`vytdl.plugin.<name>`）、`version`、`engine ^1`、`slots`、`permissions` | `packages/plugin-runtime/src/`（校验）+ 各 `vytdl/packages/plugin-*/plugin.json` |
| 权限门控 API 代理 | 插件只能调 manifest 声明的能力域（`api:downloads`、`shell:reveal-folder`…），未声明直接拒绝 | `packages/plugin-runtime/src/permissions.ts` |
| 插槽（slots） | `nav` / `page` / `settings.section` / `downloads.form-tab` / `download-detail.action`，宿主按 `order` 排序渲染 | registry + `<PluginPage>` 挂载 |
| 错误隔离 + 恢复 | 插件 activate/渲染抛错 → 置 disabled、恢复卡片列出，宿主不白屏 | `packages/plugin-runtime` lifecycle |
| Safe Mode | 只加载 `builtin: true` 的一方插件，提供移除第三方插件界面 | runtime safe mode |
| Market 安装器 | `vytdl plugin install <npm|git spec>` → `~/.vytdl/plugins/<id>/`，安装时校验 manifest，失败保留现场 | Sprint 4 |
| i18n 合并 | 各插件自带 locales，运行时合并进全局翻译表 | `packages/plugin-runtime/locales/` |

## vYtDL 改造了什么（与 DSH 的差异）

1. **后端固定，不做动态后端插件**。DSH 的插件生态在宿主服务层；vYtDL 反其道：Go daemon 只暴露稳定核心 API（下载/设置/元数据/VTT/音频），插件是**纯前端层**组合。这让 Go 侧保持简单，避开 Go 热加载的复杂度与安全风险，也把第三方插件爆炸半径锁在渲染进程内。
2. **宿主最小被写成不变量**：`apps/web` 不 import 任何具体插件；移除任一插件包，宿主必须可构建、可启动（路由渲染空态）。CI 保证。
3. **一方先行**：Sprint 1–3 不做第三方加载，先用 downloads/library/settings/analyze 四个一方插件把契约跑稳（对应 DSH 风险清单里的"先稳定契约再开放生态"）。
4. **工程定位**：`packages/plugin-runtime` 刻意放在顶层共享层（与项目无关），ContentForge 后续接插件零迁移。

## 落地过程（怎么完成的）

```
Sprint 0  设计：目标架构 + 插件模型（对照 DSH 逐条决策，产出 02-plugin-model.md）
Sprint 1  Go daemon（vytdl serve）：CLI 的 internal/* 包直接复用 + 队列/SQLite/HTTP+WS
Sprint 2  Electron Shell MVP：spawn 守护进程 + 沙箱 BrowserWindow + preload 桥
Sprint 3  插件运行时 + 一方插件拆分：manifest/权限/插槽/错误隔离/安全模式 + i18n
Sprint 4  收尾：market 安装器、AI 线下架、发布流水线
2026-10   仓库切割：vytdl/ 与 contentforge/ 两项目独立，packages/ 共享层留顶层
```

关键取舍都记录在 `docs/refactor/sprint-0/`（含风险表 R6"契约未稳先不开放第三方"、开放问题 Q1"CLI 单一源"）。

## 结果与验证

- 宿主 + 4 个一方插件（downloads / library / settings / analyze）全部走同一契约；
- 每个插件包带独立 `tsconfig.json` 严格类型检查（`packages/ui` 的历史债务隔离在外）；
- 第三方插件从 npm/git 安装、engine 主版本拒载、safe mode 可救援；
- 文档协议化：`docs/plugin-protocol.md` 作为 v1 契约规范（对应 runtime `engine: "^1"`）。

## 对下一个项目的意义

vYtDL 验证了 DSH 式「宿主 + 前端插件 + 固定后端」模式可以完整走完设计→实现→发布。ContentForge（Rust + Python）已预留接入位：起自己的 daemon，以 `plugin-contentforge-*` 插件组挂同一宿主（决策 D5）。`packages/plugin-runtime` 无需移动即可复用。
