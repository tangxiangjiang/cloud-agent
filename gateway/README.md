# gateway

Go 网关（HTTP + WebSocket）。当前：**M02** HTTP 基础（health / 配对 / tasks / slaves 占位）。

## 要求

- Go 1.22+

## 启动

```bash
cd gateway
go run .

# 固定配对码
go run . -pair-code ABCD-EFGH
# 或
# $env:GATEWAY_PAIR_CODE="ABCD-EFGH"; go run .
```

启动日志会打印 `pair code: ...`。

构建：

```bash
go build -o bin/gateway .
```

## 接口

| 方法 | 路径 | 鉴权 |
|------|------|------|
| GET | `/v1/health` | 无 |
| POST | `/v1/auth/pair` | body: `{"pairCode":"..."}` |
| GET | `/v1/auth/me` | `Authorization: Bearer <token>` |
| GET | `/v1/slaves` | Bearer；占位列表来自 YAML 配置 |
| POST | `/v1/tasks` | Bearer；可选头 `Idempotency-Key` |
| GET | `/v1/tasks` | Bearer；query `status`、`limit` |
| GET | `/v1/tasks/{id}` | Bearer |
| POST | `/v1/tasks/{id}/cancel` | Bearer；无执行器时直接 `cancelled` |
| GET | `/v1/ws` | WebSocket；query `token=` 或首帧 `{"type":"auth","token":"..."}` |

```bash
curl -s http://127.0.0.1:8080/v1/health

curl -s -X POST http://127.0.0.1:8080/v1/auth/pair \
  -H "Content-Type: application/json" \
  -d "{\"pairCode\":\"ABCD-EFGH\"}"

curl -s http://127.0.0.1:8080/v1/auth/me \
  -H "Authorization: Bearer <token>"
```

## 配置

| 来源 | 说明 |
|------|------|
| `-addr` / `GATEWAY_ADDR` | 监听地址，默认 `:8080` |
| `-pair-code` / `GATEWAY_PAIR_CODE` | 配对码；为空则启动时随机生成并打日志 |
| `-config` / `GATEWAY_CONFIG` | YAML 配置（slave 占位列表）；见 `config.example.yaml` |

```bash
copy config.example.yaml config.yaml   # Windows
# 编辑 cwd 为你的白名单路径；online 首版可为 false
go run . -config config.yaml -pair-code ABCD-EFGH
```

未传 `-config` 时 `GET /v1/slaves` 返回 `{"slaves":[]}`。`online` 目前为配置静态值，M03 再接真实登记。

Token 仅存网关内存，**不会**包含或返回 `CURSOR_API_KEY`。

## WebSocket（App）

```text
1) 配对拿到 token
2) 连接 ws://127.0.0.1:8080/v1/ws
3) {"type":"auth","token":"<token>"}  → auth.ok
4) {"type":"subscribe","taskId":"tsk_xxx","lastSeq":0} → subscribed + 补发
5) {"type":"ping"} → {"type":"pong"}
6) 服务端推送 {"type":"task.event","taskId":"...","seq":n,"at":"...","event":{...}}
```

开发注入事件（勿用于生产习惯）：

```bash
go run . -debug -pair-code ABCD-EFGH
# POST /v1/debug/tasks/{id}/events  Authorization: Bearer …
# body: {"kind":"assistant.delta","payload":{"text":"hi"}}
```

**禁止**在 WS 上传 `CURSOR_API_KEY`；只用 Gateway Bearer token。

## 测试

```bash
go test ./...
# 含 internal/ws：鉴权失败、subscribe、ping/pong、seq 递增事件
```

参见：[doc/api-outline.md](../doc/api-outline.md)、[doc/communication.md](../doc/communication.md)
