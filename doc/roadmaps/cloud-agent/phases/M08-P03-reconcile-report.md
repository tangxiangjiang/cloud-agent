# M08-P03 — 对账 warnings 与查询报告

## 目标

Gateway 将 `project_sync` 与同 `(slaveId, repoId)` 下最新 active Workflow（`bundleId=milestone:*`）对比，生成 `warnings[]`；查询 API 返回可供 UI 展示的报告。

## 范围

**允许：** `gateway/**`、api-outline / project-sync 文档补全

**禁止：** 自动修复节点状态；App 完整 UI（可先用 curl 验收）

## 上下文

- [doc/project-sync.md](../../../project-sync.md) 警告规则表
- App 续跑逻辑：同 milestone 取最新 active run

## 完成定义（DoD）

- [x] 规则：progress approved vs node 仍 ready/pending → 警告
- [x] 规则：node approved vs progress pending → 警告
- [x] 规则：dirty / 无 active workflow 等（文档所列核心项）
- [x] GET sync（或 `/sync/report`）返回 `warnings` + 并排 phase 摘要
- [x] 单测覆盖至少 2 类漂移

## 禁止事项

- 无用户确认时 PATCH 工作流节点为 approved
