# M01-P03 — 共享契约：Workflow / Review / Diff

## 目标

定义 Workflow/DAG 节点状态、`awaiting_review`、approve/revise/reject、只读 diff 响应的契约。

## 范围

**允许：** `contracts/**`、`doc/api-outline.md` 中 workflow/review 小节对齐  

**禁止：** 实现审核业务逻辑  

## 上下文

- [doc/workflow.md](../../../../workflow.md)
- [doc/api-outline.md](../../../../api-outline.md)

## 完成定义（DoD）

- [x] 节点状态含：pending/ready/running/awaiting_review/approved/rejected/failed/cancelled/skipped
- [x] revise / review(approve|reject) 请求体 schema
- [x] diff 响应：文件列表 + unified diff（或 hunks）schema
- [x] 注明：approve 前不得视为节点业务成功

## 禁止事项

- App 可写回源码的任何 API 形状
