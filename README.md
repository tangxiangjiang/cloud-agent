# cloud-agent

个人远程编码代理：**Cursor IDE 设计** → `ai/bundles` 执行计划 → **本机 Node Slave**（Cursor Local Agent）→ 手机 **触发 / 进度 / 只读 diff 审核**。

不做 Cursor Cloud Agent / 云端整仓托管。API Key 只留本机 Slave。

## 仓库布局

| 目录 | 职责 |
|------|------|
| [`gateway/`](./gateway/) | **Go** 网关：HTTP + WebSocket、鉴权、任务/工作流、推送到 App |
| [`slave/`](./slave/) | **Node (TypeScript)**：出站连 Gateway，调 `@cursor/sdk` Local runtime |
| [`app/`](./app/) | **Flutter** 手机端：触发、进度、只读 diff、修改意见、通过 |
| [`contracts/`](./contracts/) | 共享 JSON Schema / OpenAPI 契约 |
| [`doc/`](./doc/) | 架构与设计文档、roadmap（milestone / phase） |
| [`examples/`](./examples/) | 样例 DAG（如 `sample-dag.json`） |
| [`ai/`](./ai/) | AI 工作区：bundles、units、skills、进度勾选 |

## 文档入口

- 文档索引：[doc/README.md](./doc/README.md)
- **本机联调**：[doc/deploy.md](./doc/deploy.md)
- 总体架构：[doc/architecture.md](./doc/architecture.md)
- 开发计划：[doc/roadmaps/cloud-agent/README.md](./doc/roadmaps/cloud-agent/README.md)
- AI 索引：[ai/README.md](./ai/README.md)

## 状态

MVP 主干已实现（M01–M07）。联调从 [doc/deploy.md](./doc/deploy.md) 开始；检查清单：[mvp-checklist.md](./doc/roadmaps/cloud-agent/mvp-checklist.md)。
