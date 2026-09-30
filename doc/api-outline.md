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

`GET /v1/slaves` → 在线 slave、工程白名单与 milestone 索引（Slave `register` 时上报）

```json
{
  "slaves": [
    {
      "id": "slave_devpc",
      "name": "台式机",
      "online": true,
      "repos": [
        { "id": "r_cloud_agent", "name": "cloud-agent", "cwd": "E:/workspace/cloud-agent" }
      ],
      "projects": [
        {
          "id": "r_cloud_agent",
          "name": "cloud-agent",
          "cwd": "E:/workspace/cloud-agent",
          "index": "ai/milestones.json",
          "milestones": [
            {
              "id": "M07",
              "title": "硬化与收尾",
              "progressDoc": "ai/progress.md",
              "phases": [
                {
                  "id": "M07-P01",
                  "title": "审计与限流",
                  "phaseRef": "doc/roadmaps/cloud-agent/phases/M07-P01-audit-ratelimit.md",
                  "dependsOn": [],
                  "model": "composer-2.5"
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

`repos` 为 `projects` 的扁平映射（兼容旧客户端 / 任务白名单）。App 主路径：Slave → 工程 → Milestone → `POST /v1/workflows`（或续跑已有实例）。工程索引见 [`ai/milestones.md`](../ai/milestones.md)。

### 工程状态同步（M08-P01 Gateway）

见 **[project-sync.md](./project-sync.md)**。Bearer 鉴权；**不**改写 Workflow 节点 status。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/slaves/{slaveId}/projects/{repoId}/sync` | App 触发 → `202`；offline → `409`；无 WS 连接 → `503`；Gateway → Slave WS `project.sync` |
| POST | `/v1/project-sync` | Slave 上报 `{requestId,slaveId,repoId,payload}` → UPSERT `project_sync` |
| GET | `/v1/slaves/{slaveId}/projects/{repoId}/sync` | 最近快照 + 现算 `warnings[]` + `report`（phase 并排；M08-P03） |
| GET | `/v1/slaves/{slaveId}/projects/{repoId}/sync/report` | 同上（UI 友好别名） |

等价 WS：Slave → Gateway `project.sync.result`（同 payload）→ `project.sync.ok`。审计：`project.sync` / `project.sync.result`（无 token / API Key）。

GET 响应额外字段（对账，不改写节点 status）：

```json
{
  "warnings": [
    {
      "code": "progress_ahead",
      "message": "本机 progress 已标记 M01-P01 完成，但 Gateway 节点仍为 ready",
      "phaseId": "M01-P01",
      "workflowId": "wf_xxx",
      "suggestion": "continue_workflow"
    }
  ],
  "report": {
    "workflowId": "wf_xxx",
    "bundleId": "milestone:M01",
    "workflowStatus": "running",
    "active": true,
    "branch": "main",
    "dirty": false,
    "phases": [
      {
        "id": "M01-P01",
        "progressStatus": "approved",
        "nodeStatus": "ready",
        "workflowId": "wf_xxx"
      }
    ]
  }
}
```

`warnings[].code`：`progress_ahead` | `gateway_ahead` | `no_active_workflow` | `no_workflow` | `dirty_worktree`。

### 工程对话（M09-P01 Gateway）

见 **[project-chat.md](./project-chat.md)**。Bearer；**不**接受 App 传入 cwd。会话进程内存储（持久化见 M09-P04）。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/chats` | 创建会话 `{slaveId,repoId,mode?,model?}` → `201`（App 宜懒创建：首条消息时再调） |
| GET | `/v1/chats?slaveId=&repoId=` | 列表（`updatedAt` 降序；含 `title`/`preview`） |
| GET | `/v1/chats/{id}` | 会话 + messages 摘要 |
| PATCH | `/v1/chats/{id}` | `{title}` 手工改名（Sanitize） |
| POST | `/v1/chats/{id}/messages` | `{text,mode?,model?}` → `202 {taskId,chatId}`；首条用户消息写临时 `title` 并触发 Slave `chat.autotitle` |
| POST | `/v1/chats/{id}/assistant` | `{taskId?,content}` 截断写入 assistant 摘要（无 tool payload） |
| POST | `/v1/chats/{id}/stop` | 取消最近一条关联 Task（同 `POST /v1/tasks/{id}/cancel`） |
| GET | `/v1/models` | App 下拉选择列表 `{default,models[{id,label}]}`（可编辑；首启种子来自 `GATEWAY_MODELS` / 内置） |
| GET | `/v1/models/manage` | `{default,selected[],available[]}`；`available` 来自 Slave `Cursor.models.list` |
| POST | `/v1/models` | `{id,label?}` 加入选择列表 |
| DELETE | `/v1/models/{id}` | 移出选择列表（`auto` 不可删） |
| PUT | `/v1/models/default` | `{id}` 设置默认（须已在 selected） |
| POST | `/v1/models/refresh` | 请在线 Slave 重新拉取 Cursor 目录 → `models.report` |

Task 可选字段：`chatId`、`mode`、`model`（`auto` → Slave 按 `Cursor.models.list()` 解析，Pro 多为 `default`）。流式仍走 App WS / `GET /v1/tasks/{id}/events`。审计：`chat.create` / `chat.message` / `chat.stop`。

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

### 工作流（DAG 实例）

机器可读契约：`contracts/schemas/workflow-run.schema.json`、`workflow-bundle.schema.json`、`workflow-node-status.schema.json`。

**创建 / 加载 DAG 快照**（Bearer）

`POST /workflows`

```json
{
  "bundleId": "bundle_m01_demo",
  "bundleRef": "ai/bundles/m01.json",
  "slaveId": "slave_devpc",
  "repoId": "r_cloud_agent",
  "progressDoc": "ai/progress.md",
  "defaultModel": "auto",
  "defaultPolicy": { "autoApprove": false, "autoStartNext": false },
  "nodes": [
    {
      "id": "M01-P01",
      "phaseRef": "doc/roadmaps/.../M01-P01-xxx.md",
      "title": "…",
      "dependsOn": [],
      "model": "composer-2.5"
    },
    {
      "id": "M01-P02",
      "phaseRef": "…",
      "dependsOn": ["M01-P01"]
    }
  ]
}
```

→ `201` WorkflowRun：`status=pending`；无依赖节点为 `ready`，其余 `pending`。  
节点默认可带 `model`（默认 `auto`）与 `policy`（`autoApprove`/`autoStartNext`，**默认均为 false**）。  
未写的节点字段继承请求级 `defaultModel` / `defaultPolicy`（App「执行 plan」批量默认）。  
**环检测：** Gateway 拒绝含环或未知 `dependsOn` 的图（HTTP 400）。客户端应只提交已校验的 DAG。

`GET /workflows?status=&limit=` → `{ "workflows": [ … ] }`  
`GET /workflows/{workflowId}` → 完整 Run（含 `nodes`、`dependsOn`、节点状态）  
`GET /workflows/{workflowId}/nodes` → `{ "nodes": [ … ] }`

**启动 / 续跑调度**（Bearer）

`POST /workflows/{workflowId}/start` → `{ "workflow", "delivered" }`  
首次将 Run 标为 `running`，并向 `slaveId` 出站推送 `workflow.assign`。

`POST /workflows/{workflowId}/continue` → 有 `ready` 节点时再次 `AssignWorkflow`（Approve 后默认**不会**自动续跑，须显式 continue 或节点 `policy.autoStartNext`）。

`POST /workflows/{workflowId}/nodes/{nodeId}/start` → 仅当该节点为 `ready` 时 assign。

`POST /workflows/{workflowId}/nodes/{nodeId}/reset` → 仅当节点为 `failed` / `rejected` / `cancelled`：清 `taskId`、回到 `ready`（依赖未齐则为 `pending`），并把 workflow 从 `failed` 恢复为 `running`（已通过的上游不动）。  
Body 可选 `{ "start": true }`：节点变为 `ready` 时立即 `AssignWorkflow`。

**节点状态 / 策略同步**（Bearer；Slave 在跑节点时调用）

`PATCH /workflows/{workflowId}/nodes/{nodeId}`

```json
{ "status": "awaiting_review", "taskId": "tsk_…", "model": "auto", "policy": { "autoApprove": false, "autoStartNext": false } }
```

Agent 成功只许进入 `awaiting_review`，**不得**直接 `approved`。仅当依赖节点均为 `approved` 时，下游才变为 `ready`（审核闸门）。  
**Approve 后续跑：** 默认停在下游 `ready`；仅当被批节点 `policy.autoStartNext==true` 才自动 `AssignWorkflow`（见 [node-policy.md](./node-policy.md)）。  
样例 DAG：`examples/sample-dag.json`、`slave/fixtures/dag-two-node.json`、`ai/bundles/m05-p02-two-node.json`。联调：[deploy.md](./deploy.md)。

节点状态枚举：`pending / ready / running / awaiting_review / approved / rejected / failed / cancelled / skipped`。  
审核 / diff：只读 diff 见 M05-P03；revise 见 M05-P04；approve 写进度见 M05-P05（`POST .../review`，Slave 仅 approve 时更新 `progressDoc`）。

### 工作流节点审核（App 闸门）

节点须为 `awaiting_review`。  
**硬约定：** Agent 跑完进入 `awaiting_review` **不等于**节点业务成功；只有 `approve` 后才是 `approved`，方可写进度文档并解锁下游。机器可读契约见 [`contracts/schemas/`](../contracts/schemas/)（M01-P03）。

**通过**

`POST /workflows/{workflowId}/nodes/{nodeId}/review`

```json
{ "decision": "approve", "comment": "可选" }
```

→ Slave 更新进度文档 → `approved` → 解锁下游为 `ready`。  
**默认不**再次 `workflow.assign`；若该节点 `policy.autoStartNext==true` 则自动续跑，否则用 `POST …/continue` 或 `POST …/nodes/{id}/start`。

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

`GET /workflows/{workflowId}/nodes/{nodeId}/diff`（Bearer）  
→ 文件列表 + `unifiedDiff`（或 hunks），供 App **只读渲染**。  
Schema：`contracts/schemas/node-diff.schema.json`。

Slave 上传（非 App）：`PUT` 同一路径，body 为 NodeDiff。  
**无** apply-patch / 写回源码接口。基线策略：节点开始时 `git rev-parse HEAD` → `baseline: "git:<sha>"`，详见 [slave/docs/diff-baseline.md](../slave/docs/diff-baseline.md)。

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

Node Slave **出站**连 Go Gateway WebSocket，不反向对公网开端口。

- URL：`GET /v1/slave/ws`（升级为 WS）  
- 消息：`auth` / `register` / `heartbeat`（详见 [gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)）  
- `register` 后 `GET /v1/slaves` 中对应项 `online=true`；断开经宽限后 `online=false`  

后续（M03-P03+）还将要求：

1. 事件带 `taskId` + `seq`  
2. 终态落到 Gateway 存储  
3. 取消指令能传到 `run.cancel()`（若 `supports("cancel")`）
