# gateway

Go 网关（HTTP + WebSocket）。当前里程碑：**M02-P01** 骨架 + 健康检查。

## 要求

- Go 1.22+（使用标准库路由 `GET /v1/health` 语法）

## 启动

```bash
cd gateway
go run .

# 或指定端口
go run . -addr :8080
# / 环境变量
# Windows PowerShell: $env:GATEWAY_ADDR=":9090"; go run .
```

构建：

```bash
go build -o bin/gateway .
```

## 健康检查

```bash
curl http://127.0.0.1:8080/v1/health
# {"ok":true}
```

## 配置

| 来源 | 说明 |
|------|------|
| `-addr` | 监听地址，默认 `:8080` |
| `GATEWAY_ADDR` | 同上；未传 flag 默认值时使用 |

参见：[doc/api-outline.md](../doc/api-outline.md)
