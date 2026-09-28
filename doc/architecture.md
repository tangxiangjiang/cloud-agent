# 总体架构

## 目标

用手机远程**观察与驱动已设计好的工作流**，在**开发者本人机器**上由 Cursor Local Agent 按 DAG 节点执行改代码；过程可观察、可取消。

设计与拆解在 Cursor IDE 由人完成（架构 → roadmap → phase），可执行计划与任务单元落在 **[`ai/`](../ai/README.md)**，再交 Slave / 手机。详见 [workflow.md](./workflow.md)、[workflow-ingest.md](./workflow-ingest.md)。

非目标：

- 不使用 Cursor Cloud Agent / 云端 VM 整仓 clone
- 不依赖 Cursor IDE UI 常开才能**执行**（IDE 用于设计段；执行靠 Local Slave）
- 不做完整 IDE 体验复刻（无 Tab / Inline Edit）
- 不在手机上做架构设计（手机消费工作流，不生产里程碑）

## 角色

```
┌─────────────┐     HTTPS / WSS      ┌──────────────────┐
│ Flutter App │ ───────────────────► │  Gateway (Go)    │
│  (手机)     │ ◄─── 事件推送 ────── │                  │
└─────────────┘                      └────────┬─────────┘
                                              │ Slave 出站连接
                                              ▼
                                     ┌──────────────────┐
                                     │ Local Slave      │
                                     │ (Node + SDK)     │
                                     │ local.cwd=仓库   │
                                     └──────────────────┘
```

| 角色 | 职责 | 语言 |
|------|------|------|
| Flutter App | **触发 + 进度 + 审核**（不做设计/拆 phase） | Dart |
| Gateway | 鉴权、工作流/任务、审核中转、向 App 推事件、审计 | **Go** |
| Local Slave | 调 `@cursor/sdk` 编码；**仅 App 批准后**更新进度文档 | **Node (TS)** |

分工原则：Go 只做网关与编排；**只有 Node Slave 调用官方 SDK**。

## 数据流（两条）

### A. 工作流（主路径）

1. **IDE（人+AI）** 产出 milestone / phase / DAG（设计段不在手机）  
2. DAG 经 Gateway 下发或 Slave 加载；App 可见快照  
3. App **触发** start  
4. Slave 跑节点 → `awaiting_review`，推送变更摘要  
5. App **审核** approve / reject  
6. **仅 approve 后**：Slave **更新进度文档**，节点 `approved`，下游解锁  
7. 全部节点批准后工作流完成

### B. 单次自由任务（快捷入口）

1. App `POST /tasks` 提交 prompt + 目标 slave/仓库  
2. Gateway 入队 → Slave `agent.send` → 流式回传 → 终态  
3. 跟进：`POST /tasks/{id}/follow-ups`

## 安全约束（个人开发前提）

| 原则 | 做法 |
|------|------|
| 整仓不上云执行环境 | 仅 Local runtime；禁止 Cloud Agent 代码路径 |
| API Key 不进手机 | `CURSOR_API_KEY` 只存在 Node Slave / 本机环境 |
| 手机只信自己的网关 | App 持有 Gateway 签发的设备/用户 token |
| 仓库白名单 | Slave 只允许配置过的 `cwd` 列表 |
| 工具闸门 | hooks / sandbox 限制危险 shell（后续迭代） |
| 审计 | 记录 `agentId`、`runId`、任务文本摘要、终态 |

说明：Local 仍会把任务相关**代码片段/上下文**发给 Cursor 托管模型做推理——与「整仓落在云端 VM」不同。个人相对云厂商法务不对等，故用架构规避整仓托管风险。

## 部署形态（个人）

推荐初期：

- Gateway（Go）与 Slave（Node）**同机或同内网**（例如家里 NAS / 开发机）
- App 经公网访问时：Gateway 暴露 HTTPS/WSS（Tailscale / Cloudflare Tunnel / 自有域名均可）
- Slave **不**对公网暴露，只出站连 Gateway

## 与 Cursor IDE 的关系

| 能力 | Local Slave | Cursor IDE |
|------|-------------|------------|
| Agent 改代码 / 跑命令 / 多轮 | ✅ | ✅ |
| 编辑器 UI、Tab、Inline | ❌ | ✅ |
| 无人值守、手机驱动 | ✅ | 弱 |

能力对齐目标：**IDE 里 Agent 的编码主循环**，不是整个 IDE。
