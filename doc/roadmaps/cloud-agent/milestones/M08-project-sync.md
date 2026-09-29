# M08 — 工程状态同步

## 目标

App 在「Slave → 工程」触发同步：Slave 采集本机 milestone / progress / git（可选 AI 摘要），Gateway 写入 `project_sync` 并与 active Workflow 对账，避免工程磁盘状态与 Gateway 工作流状态长期漂移。

设计权威文档：[../../project-sync.md](../../project-sync.md)。

## 非目标

- 不自动 Approve、不自动改 progress.md / 节点 status（无显式「修复」确认前）  
- 不做 Redis / 多 Gateway 集群  
- 不同步完整源码 diff 或 API Key  

## 依赖

M07（SQLite 持久化与 `project_sync` 表预留已具备）

## 验收

- App 工程页可点「同步」，Slave online 时完成一轮采集上报  
- Gateway SQLite `project_sync` 有对应 `(slaveId, repoId)` 行  
- 能展示 progress vs Workflow 的 warnings（至少规则对比）  
- 审计能查到 sync accept / result；无 Key 明文  

## Phases

| Phase | 文档 |
|-------|------|
| M08-P01 | [Gateway 同步 API 与落库](../phases/M08-P01-gateway-sync-api.md) |
| M08-P02 | [Slave 采集与上报](../phases/M08-P02-slave-collect.md) |
| M08-P03 | [对账 warnings 与查询报告](../phases/M08-P03-reconcile-report.md) |
| M08-P04 | [App 工程页同步 UI](../phases/M08-P04-app-sync-ui.md) |
| M08-P05 | [可选 AI 摘要](../phases/M08-P05-ai-summary.md) |
