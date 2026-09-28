# Approve 写 progress（M05-P05）

## 流程

```
awaiting_review
  → App POST /v1/workflows/{id}/nodes/{nodeId}/review { "decision": "approve" }
  → Gateway: 节点 → approved；RecomputeReady（下游 pending→ready）
  → WS workflow.review (approve) + workflow.assign
  → Slave: 仅此时更新 Bundle.progressDoc（basename 必须为 progress.md）
  → Slave: 继续调度新 ready 节点
```

`decision: reject` → 节点 `rejected`，**不**写 progress，下游不解锁。

## 路径约束

| 规则 | 说明 |
|------|------|
| 来源 | 仅 `WorkflowRun.progressDoc`（创建工作流时声明） |
| 相对路径 | 相对仓库白名单 `cwd`，禁止绝对路径与 `..` 逃逸 |
| 文件名 | basename 必须为 `progress.md`（防止改 phase 设计正文） |

## 勾选格式

从节点 `id` / `unitId` / `phaseRef` 文件名解析 `Mxx-Pxx`，写入：

```markdown
- [x] M05-P05 @approved 2026-09-28T12:00:00.000Z
```

若存在 `| M05-P05 | … |` 表格行，将状态格改为 `approved`。不改 milestone 目标文案、不改其他设计文档。
