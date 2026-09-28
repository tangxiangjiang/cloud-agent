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

## 测试

```bash
go test ./...
```

参见：[doc/api-outline.md](../doc/api-outline.md)
