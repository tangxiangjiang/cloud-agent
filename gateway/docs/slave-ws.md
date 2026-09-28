# Slave ↔ Gateway 出站协议（M03-P02）

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
| `register` | `slaveId`, `name?`, `repos[{id,name,cwd}]` | 登记；Gateway 将 `online=true` |
| `heartbeat` | — | 保活；刷新 online |
| `ping` | — | 应用层 ping |

### Gateway → Slave

| type | 字段 | 说明 |
|------|------|------|
| `auth.ok` | — | 鉴权成功 |
| `registered` | `slaveId` | 登记成功 |
| `heartbeat.ok` | — | 心跳应答 |
| `pong` | — | 对 `ping` |
| `error` | `error` | 失败 |

后续 M03-P03 将增加任务下发 / 事件上报消息，本阶段仅登记与在线态。

## 在线语义

- 配置文件中的 `online` **忽略**；仅出站 `register` 后为 online  
- 连接断开后经宽限（默认 3s）置 `online=false`  
- `GET /v1/slaves`（App Bearer）反映上述状态  

## 最小客户端

```bash
# 终端 A
cd gateway && go run . -pair-code ABCD-EFGH -debug

# 终端 B：配对拿 token，再
go run ./cmd/slaveping -token <token> -id slave_devpc -cwd /path/to/repo
```

然后 App/`curl` 带 Bearer 访问 `GET /v1/slaves` 应见 `online: true`。
