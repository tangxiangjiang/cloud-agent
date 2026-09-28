# gateway

Go 网关（HTTP + WebSocket）。当前：**M02-P02** 配对鉴权。

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
| POST | `/v1/tasks` | Bearer；可选头 `Idempotency-Key` |
| GET | `/v1/tasks` | Bearer；query `status`、`limit` |
| GET | `/v1/tasks/{id}` | Bearer |
| POST | `/v1/tasks/{id}/cancel` | Bearer；无执行器时直接 `cancelled` |

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

Token 仅存网关内存，**不会**包含或返回 `CURSOR_API_KEY`。

## 测试

```bash
go test ./...
```

参见：[doc/api-outline.md](../doc/api-outline.md)
