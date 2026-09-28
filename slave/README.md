# slave

Node.js / TypeScript **Local Slave**：出站连接 Go Gateway，用官方 `@cursor/sdk` **Local runtime** 执行任务（禁止 Cloud Agent / `cloud`）。

## 要求

| 依赖 | 版本 / 说明 |
|------|-------------|
| **Node.js** | **>= 22.13**（`@cursor/sdk` engines；当前机若仍是 18 请先升级） |
| npm | 随 Node 自带即可 |
| Gateway Bearer | 环境变量（默认 `GATEWAY_TOKEN`），来自 `POST /v1/auth/pair` |
| Cursor API Key | 环境变量（默认 `CURSOR_API_KEY`），**不要**写入配置文件 |
| Gateway | 本机或可达的 cloud-agent Gateway |

## 快速开始

```bash
# 终端 A：Gateway
cd gateway && go run . -pair-code ABCD-EFGH

# 终端 B：配对
curl -s -X POST http://127.0.0.1:8080/v1/auth/pair \
  -H "Content-Type: application/json" \
  -d "{\"pairCode\":\"ABCD-EFGH\"}"

# 终端 C：Slave（Node >= 22.13）
cd slave
npm install
cp config.example.yaml config.yaml
# 编辑 repos[].cwd 为绝对路径白名单

# PowerShell 示例（勿把真实值提交进 git）
$env:GATEWAY_TOKEN = "<pair-token>"
$env:CURSOR_API_KEY = "<cursor-api-key>"

npm run build
npm start -- --config config.yaml
```

仅联调 Gateway、不跑 SDK：`npm start -- --stub`

开发：`npm run dev -- --config config.yaml`  
单测：`npm test`

### 本地 smoke（无 Gateway，验证 SDK Local）

```bash
# Node >= 22.13；CURSOR_API_KEY 必填
# 默认 cwd = fixtures/smoke-repo
$env:CURSOR_API_KEY = "<cursor-api-key>"
npm run smoke-local

# 可选覆盖
# $env:SMOKE_CWD = "E:/workspace/some-repo"
# $env:SMOKE_MODEL = "composer-2.5"
# $env:SMOKE_PROMPT = "Reply with exactly: smoke-ok."
```

成功时日志含 stream 映射的 `assistant.delta` / `tool.*`，并以 `status: finished` 结束。启动失败（鉴权等）exit 1；run 失败 exit 2。

## 行为

1. 校验配置与 `repos[].cwd` 绝对路径白名单；拒绝配置中的 `cloud`
2. 出站 WS：`auth` → `register` → `heartbeat`；断线重连
3. `task.assign` → `Agent.create({ local: { cwd, settingSources: [] }, model, apiKey })` → `send` → `stream` 映射契约事件 → **必须** `wait` → dispose  
   - 无 `cloud` 字段；默认不加载 `settingSources: "all"`
4. 错误区分：`CursorAgentError` → `phase: startup`；`result.status === "error"` → `phase: run`
5. 日志脱敏，不打印 API Key / Bearer

## 配置字段

| 字段 | 说明 |
|------|------|
| `gatewayUrl` | Slave 出站 WS |
| `slaveId` | 与创建任务一致 |
| `repos[]` | `id` / `name` / `cwd`（绝对路径） |
| `apiKeyEnv` | Cursor API Key 环境变量名（默认 `CURSOR_API_KEY`） |
| `tokenEnv` | Gateway Bearer 环境变量名（默认 `GATEWAY_TOKEN`） |
| `defaultModel` | task 未带 model 时使用（默认 `composer-2.5`） |

## Roadmap

| Phase | 内容 |
|-------|------|
| M04-P01 | 工程 + 配置 / cwd 白名单 |
| M04-P02 | Gateway 出站客户端 |
| **M04-P03** | Local Agent create/send/stream/wait（当前） |
| M04-P04 | 取消 / 错误 / 安全默认加强 |

协议：[../gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)  
设计：[doc/local-slave.md](../doc/local-slave.md)
