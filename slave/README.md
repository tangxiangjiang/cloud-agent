# slave

Node.js / TypeScript **Local Slave**：出站连接 Go Gateway，调用官方 `@cursor/sdk`（仅 Local runtime，禁止 Cloud Agent）。

## 要求

| 依赖 | 版本 / 说明 |
|------|-------------|
| **Node.js** | **>= 18**（engines 与官方 SDK 一致；建议 LTS） |
| npm | 随 Node 自带即可 |
| Gateway Bearer | 环境变量（默认 `GATEWAY_TOKEN`），来自 `POST /v1/auth/pair` |
| Cursor API Key | 环境变量（默认 `CURSOR_API_KEY`），M04-P03 起需要；**不要**写入配置文件 |
| Gateway | 本机或可达的 cloud-agent Gateway |

## 快速开始（M04-P02：出站登记）

```bash
# 终端 A：Gateway
cd gateway && go run . -pair-code ABCD-EFGH

# 终端 B：配对拿 token
curl -s -X POST http://127.0.0.1:8080/v1/auth/pair \
  -H "Content-Type: application/json" \
  -d "{\"pairCode\":\"ABCD-EFGH\"}"
# → {"token":"..."} 写入环境变量 GATEWAY_TOKEN（勿提交）

# 终端 C：Slave
cd slave
npm install
cp config.example.yaml config.yaml
# 编辑 repos[].cwd 为绝对路径；slaveId 与创建任务时一致

set GATEWAY_TOKEN=<token>   # Windows PowerShell: $env:GATEWAY_TOKEN="..."
npm run build
npm start -- --config config.yaml

# GET /v1/slaves 应见 online=true（同一 Bearer）
# POST /v1/tasks {"slaveId":"slave_devpc","repoId":"r_cloud_agent","prompt":"hi"}
# stub 会回报 status/running → assistant.delta → done（尚无 SDK）
```

开发：`npm run dev -- --config config.yaml`  
单测：`npm test`（配置校验 + mock WS 登记/下发/重连）

配置示例：`config.example.yaml`、密钥占位：`.env.example`（均不含真实 Key）。

## 行为

1. 加载 YAML，校验 `repos[].cwd` 绝对路径白名单；拒绝 `cloud`
2. 出站连接 `gatewayUrl`，首帧 `auth`（token **不入日志**）→ `register` → 周期 `heartbeat`
3. 收到 `task.assign`：stub 回传至少 `status`（及 delta/done）；`task.cancel` 标记 cancelled
4. 断线后指数退避重连并重新登记
5. 结构化日志脱敏（长 token / `token` 字段名）

本进程**不**对公网开 HTTP；真实 `@cursor/sdk` Local Agent 见 M04-P03。

## 配置字段

| 字段 | 说明 |
|------|------|
| `gatewayUrl` | Slave 出站 WS，如 `ws://127.0.0.1:8080/v1/slave/ws` |
| `slaveId` | 与 Gateway / App 创建任务时一致 |
| `repos[]` | `id` / `name` / `cwd`（绝对路径） |
| `apiKeyEnv` | Cursor API Key 的环境变量名（默认 `CURSOR_API_KEY`） |
| `tokenEnv` | Gateway Bearer 的环境变量名（默认 `GATEWAY_TOKEN`） |

## Roadmap

| Phase | 内容 |
|-------|------|
| M04-P01 | 工程 + 配置加载 / cwd 白名单 |
| **M04-P02** | Gateway 出站客户端（当前） |
| M04-P03 | `@cursor/sdk` Local Agent |
| M04-P04 | 取消 / 错误 / 安全默认 |

协议：[../gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)  
设计：[doc/local-slave.md](../doc/local-slave.md)
