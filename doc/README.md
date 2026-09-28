# cloud-agent 文档

个人向远程编码代理：设计在 **Cursor IDE（人+AI）**；执行由 **Node Slave + Local Agent**；手机是 **触发器 + 进度 + 审核**（只读 diff，输入框发修改意见，通过后才写进度文档）。  
不做 Cloud Agent / 云端整仓托管。

## 文档索引

| 文档 | 内容 |
|------|------|
| [deploy.md](./deploy.md) | **本机部署联调**：环境变量、配对、启动顺序、样例 DAG |
| [architecture.md](./architecture.md) | 总体架构、角色边界、安全约束 |
| [workflow.md](./workflow.md) | **工作流**：人工设计 → milestone/phase → DAG → Slave/App |
| [workflow-ingest.md](./workflow-ingest.md) | **投喂**：Skill → **`ai/`** → Slave → 手机任务单元 |
| [../ai/README.md](../ai/README.md) | **AI 工作区索引**（bundles / units / skills） |
| [communication.md](./communication.md) | **HTTP vs HTTP+WebSocket** 选型结论与协议约定 |
| [flutter-app.md](./flutter-app.md) | Flutter 终端职责、页面与接口使用方式 |
| [api-outline.md](./api-outline.md) | 代理服务 HTTP / WebSocket 接口草案 |
| [local-slave.md](./local-slave.md) | 本机 Node Slave 职责与部署要点 |
| [roadmaps/cloud-agent/](./roadmaps/cloud-agent/README.md) | **开发计划**：Milestone → Phase |

## 技术选型（当前冻结）

| 层 | 选型 |
|----|------|
| 手机 App | Flutter |
| Gateway | **Go**（HTTP + WebSocket、鉴权、任务队列） |
| Local Slave | **Node（TypeScript）** + 官方 `@cursor/sdk` Local runtime |
| Agent 执行 | 仅 Local（`local.cwd`）；**不做** Cloud Agent |
| 通信 | **HTTP + WebSocket 混合**（见 communication.md） |

## 快速结论：通信要不要 WebSocket？

**要混合。** HTTP 负责鉴权、建任务、查询、取消；WebSocket 负责任务过程的实时推送（assistant 文本、工具事件、状态变更）。  
纯 HTTP 轮询能做 MVP，但 Agent 流式体验差、耗电与延迟都差，不适合作为正式方案。
