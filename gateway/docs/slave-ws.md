# Slave ↔ Gateway 出站协议

Slave **主动出站**连接 Gateway，不对公网开入站端口。

## URL

```
ws://{gateway}/v1/slave/ws
wss://{gateway}/v1/slave/ws
```

鉴权（二选一）：

- Query：`?token=<gateway_bearer>`（来自 `POST /v1/auth/pair`，**不是** `CURSOR_API_KEY`）
- 首帧：`{"type":"auth","token":"<gateway_bearer>"}`

## 消息类型

### Slave → Gateway

| type | 字段 | 说明 |
|------|------|------|
| `auth` | `token` | 鉴权 |
| `register` | `slaveId`, `name?`, `repos[{id,name,cwd}]` | 登记；`online=true`；并领取该 slave 的 queued tasks |
| `heartbeat` | — | 保活 |
| `ping` | — | 应用层 ping |
| `task.event` | `taskId`, `event.kind`, `event.payload` | 上报事件；Gateway 赋 `seq` 并 fan-out 到 App WS |

### Gateway → Slave

| type | 字段 | 说明 |
|------|------|------|
| `auth.ok` | — | 鉴权成功 |
| `registered` | `slaveId` | 登记成功 |
| `heartbeat.ok` | — | 心跳应答 |
| `pong` | — | 对 `ping` |
| `task.assign` | `task`（Task JSON） | 下发 queued 任务（无 workflowId 的自由任务） |
| `task.cancel` | `taskId` | 取消 |
| `workflow.assign` | `workflow`（WorkflowRun JSON） | `POST /workflows/{id}/start` 或 approve 后续跑；串行 DAG 调度 |
| `workflow.revise` | `workflowId`, `nodeId`, `instruction` | App revise；Slave follow-up / 再跑（不写 progress） |
| `workflow.review` | `workflowId`, `nodeId`, `decision`, `comment?` | `approve` → Slave 写 `progressDoc`；`reject` → 不写 |
| `error` | `error` | 失败 |

`task.event.kind` 与 App 侧一致：`status` / `assistant.delta` / `tool.*` / `error` / `done`。

## 在线语义

- 配置文件中的 `online` **忽略**；仅出站 `register` 后为 online  
- 连接断开后经宽限（默认 3s）置 `online=false`  
- `GET /v1/slaves`（App Bearer）反映上述状态  

## 联调

**Node Slave（推荐，M04-P02+ stub 事件）：**

```bash
# 终端 A
cd gateway && go run . -pair-code ABCD-EFGH

# 终端 B：配对 → 将 token 设为 GATEWAY_TOKEN
curl -s -X POST http://127.0.0.1:8080/v1/auth/pair -H "Content-Type: application/json" -d "{\"pairCode\":\"ABCD-EFGH\"}"

# 终端 C
cd ../slave && cp config.example.yaml config.yaml
# 编辑 repos cwd；然后：
# GATEWAY_TOKEN=<token> npm start
```

**Go mockslave（无 Node）：** `go run ./cmd/mockslave -token <token> -id slave_devpc`

创建任务后 App 订 `/v1/ws`，或 `GET /v1/tasks/{id}/events?afterSeq=0`。
