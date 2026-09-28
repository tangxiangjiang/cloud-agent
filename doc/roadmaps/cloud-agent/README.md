# Roadmap: cloud-agent

个人远程编码代理：IDE 设计 → DAG → Node Slave（Local Agent）→ App 触发/进度/**只读 diff 审核 + 修改意见** → 批准后写进度文档。

## 里程碑总览

| ID | 里程碑 | 目标一句话 | 依赖 |
|----|--------|------------|------|
| [M01](./milestones/M01-repo-contracts.md) | 仓库骨架与契约 | monorepo 目录、共享 schema、CI 空壳 | — |
| [M02](./milestones/M02-gateway-http.md) | Gateway HTTP 基础 | Go 网关：健康检查、配对鉴权、Task CRUD | M01 |
| [M03](./milestones/M03-gateway-ws-slave-protocol.md) | Gateway WS + Slave 协议 | App WS 推送；Slave 出站登记与任务下发通道 | M02 |
| [M04](./milestones/M04-slave-local-agent.md) | Node Slave + Local Agent | 单任务跑通 `@cursor/sdk` local，事件回传 | M03 |
| [M05](./milestones/M05-workflow-review-gate.md) | 工作流 DAG + 审核闸门 | workflow/node 状态、diff、revise、approve、进度文档 | M04 |
| [M06](./milestones/M06-flutter-app.md) | Flutter App MVP | 配对、触发、进度、只读 diff、意见框、通过 | M05 |
| [M07](./milestones/M07-hardening.md) | 硬化与收尾 | 审计、限流、部署说明、样例 DAG | M06 |

## 进度

见 [progress.md](./progress.md)（由人工或 **App 批准后 Slave** 更新；设计正文不在此文件改）。

## Phase 索引

各 milestone 下 phase 文件在 [phases/](./phases/)。命名：`M{nn}-P{nn}-{slug}.md`。

执行约定见 [../../workflow.md](../workflow.md)。  
可执行 Bundle / 手机任务单元落在仓库 **[`ai/`](../../../ai/README.md)**（非本目录）。

## 非目标（全 roadmap）

- Cursor Cloud Agent / 云端整仓托管  
- App 上编辑源码或编辑 DAG/phase  
- 手机做架构设计  
