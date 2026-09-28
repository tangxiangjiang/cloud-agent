# 代理服务 API 草案

Base path：`/v1`  
鉴权：`Authorization: Bearer <token>`（WS 可用 query `token` 或首帧 auth）

> 草案级别：字段名可在实现时微调，职责不要改。  
> Task / 事件信封的机器可读契约：[`contracts/schemas/`](../contracts/schemas/)（M01-P02）。

## HTTP

### 健康检查

`GET /health` → `{ "ok": true }`

### 配对（个人场景）

`POST /auth/pair`

```json
{ "gatewayUrl": "optional", "pairCode": "XXXX-XXXX" }
```

→ `{ "token": "...", "expiresAt": "..." }`

### Slaves（本机 Node 执行端）

`GET /slaves` → 在线 slave 与仓库列表  
（实现期若暂用 `/workers` 亦可，语义等同 Local Slave）

```json
{
  "slaves": [
    {
      "id": "slave_devpc",
      "name": "台式机",
      "online": true,
      "repos": [
        { "id": "r_cloud_agent", "name": "cloud-agent", "cwd": "E:/workspace/cloud-agent" }
      ]
    }
  ]
}
```

### 创建任务

`POST /tasks`  
Header 可选：`Idempotency-Key: <uuid>`

```json
{
  "slaveId": "slave_devpc",
  "repoId": "r_cloud_agent",
  "prompt": "给登录接口补上参数校验并写单测",
  "model": "composer-2.5"
}
```

→ `201`

```json
{
  "id": "tsk_xxx",
  "status": "queued",
  "agentId": null,
  "runId": null,
  "createdAt": "..."
}
```

### 任务详情

`GET /tasks/{id}`

```json
{
  "id": "tsk_xxx",
  "status": "running",
  "slaveId": "slave_devpc",
  "repoId": "r_cloud_agent",
  "prompt": "...",
  "agentId": "local_...",
  "runId": "run_...",
  "resultSummary": null,
  "error": null,
  "updatedAt": "..."
}
```

### 任务列表

`GET /tasks?status=&limit=`

### 跟进（同一会话）

`POST /tasks/{id}/follow-ups`

```json
{ "prompt": "再补一个失败用例" }
```

→ 新 `run` 挂到同一 `agentId`（Slave `resume` 或复用句柄）

### 取消

`POST /tasks/{id}/cancel` → `{ "status": "cancelling" }`

### 工作流节点审核（App 闸门）

节点须为 `awaiting_review`。  
**硬约定：** Agent 跑完进入 `awaiting_review` **不等于**节点业务成功；只有 `approve` 后才是 `approved`，方可写进度文档并解锁下游。机器可读契约见 [`contracts/schemas/`](../contracts/schemas/)（M01-P03）。

**通过**

`POST /workflows/{workflowId}/nodes/{nodeId}/review`

```json
{ "decision": "approve", "comment": "可选" }
```

→ Slave 更新进度文档 → `approved` → 解锁下游。

**按意见修改（输入框）**

`POST /workflows/{workflowId}/nodes/{nodeId}/revise`

```json
{
  "instruction": "把错误码改成与 api-outline 一致，并补一个单测"
}
```

→ Slave 对当前会话 follow-up / 再跑 Agent → 节点回 `running`，结束后再 `awaiting_review`，并刷新只读 diff。  
（请求体只有 `instruction` 文本，**无**源码写回 / apply-patch API。）

**驳回（可选）**

```json
{ "decision": "reject", "comment": "…" }
```

→ `rejected`，不写进度文档。

**只读 diff 数据**

`GET /workflows/{workflowId}/nodes/{nodeId}/diff`  
→ 文件列表 + unified diff（或按文件分页的 hunks），供 App **只读渲染**，无写回接口。  
Schema：`contracts/schemas/node-diff.schema.json`。

### 事件快照（WS 兜底）

`GET /tasks/{id}/events?afterSeq=0`

```json
{
  "events": [
    { "seq": 1, "at": "...", "kind": "status", "payload": { "status": "running" } }
  ],
  "latestSeq": 1
}
```

## WebSocket（App ↔ Gateway）

`GET /v1/ws` 升级为 WS。

### 客户端 → 服务端

```json
{ "type": "auth", "token": "..." }
```

```json
{ "type": "subscribe", "taskId": "tsk_xxx", "lastSeq": 0 }
```

```json
{ "type": "unsubscribe", "taskId": "tsk_xxx" }
```

```json
{ "type": "ping" }
```

### 服务端 → 客户端

```json
{ "type": "pong" }
```

```json
{
  "type": "task.event",
  "taskId": "tsk_xxx",
  "seq": 12,
  "at": "...",
  "event": {
    "kind": "assistant.delta",
    "payload": { "text": "准备修改 auth 模块…" }
  }
}
```

`event.kind` 枚举（首版）：

| kind | payload 要点 |
|------|----------------|
| `status` | `status` |
| `assistant.delta` | `text` |
| `tool.started` | `name`, `summary?` |
| `tool.finished` | `name`, `ok` |
| `error` | `message`, `retryable?` |
| `done` | `status` (`finished` \| `error` \| `cancelled`) |

## Slave ↔ Gateway（内部）

Node Slave **出站**连 Go Gateway（WS 或 HTTP 长拉），不反向对公网开端口。

要求：

1. 事件带 `taskId` + `seq`  
2. 终态必须落到 Gateway 存储  
3. 取消指令能传到正在跑的 `run.cancel()`（若 `supports("cancel")`）

对 App 只暴露上文公网 API；Slave 内部协议实现阶段再定。
