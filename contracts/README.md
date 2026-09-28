# contracts

Gateway / Slave / App 共用的**机器可读**契约（JSON Schema）。

对齐文档：[doc/api-outline.md](../doc/api-outline.md)、[doc/communication.md](../doc/communication.md)、[doc/workflow.md](../doc/workflow.md)、[ai/SCHEMA.md](../ai/SCHEMA.md)。

## 布局

```
contracts/
  schemas/
    # M01-P02 Task / 事件
    task-status.schema.json
    task.schema.json
    task-event-kind.schema.json
    task-event.schema.json
    # M01-P03 Workflow / Review / Diff
    workflow-node-status.schema.json
    workflow-bundle.schema.json
    workflow-run.schema.json
    review-request.schema.json
    revise-request.schema.json
    node-diff.schema.json
    task-unit.schema.json
  examples/
  scripts/validate-examples.mjs
```

## 审核闸门（硬约定）

- 节点 Agent 跑完 → `awaiting_review`，**不算**业务成功  
- 仅 App `decision: approve` 后 → `approved`，才写 `progressDoc`、解锁下游  
- `revise` 只传自然语言 `instruction`，**没有** App 写回源码 / apply-patch 的请求 schema  
- `node-diff` 仅供只读展示  

## 如何校验

```bash
npm install --no-save ajv ajv-formats
node contracts/scripts/validate-examples.mjs
```

## 约定

- 时间字段：ISO-8601 UTC（`date-time`）
- Task 事件 `seq`：同一 `taskId` 下从 1 起单调递增
- Bundle `phaseRef`：仓库相对路径，禁止 `..`
- 不做 Cloud Agent 专用状态
