# slave

Node.js / TypeScript **Local Slave**：出站连接 Go Gateway，调用官方 `@cursor/sdk`（仅 Local runtime，禁止 Cloud Agent）。

## 要求

| 依赖 | 版本 / 说明 |
|------|-------------|
| **Node.js** | **>= 18**（engines 与官方 SDK 一致；建议 LTS） |
| npm | 随 Node 自带即可 |
| Cursor API Key | 环境变量（默认名 `CURSOR_API_KEY`），**不要**写入配置文件 |
| Gateway | 本机或可达的 cloud-agent Gateway（M04-P02 起连接） |

## 快速开始（M04-P01：配置校验）

```bash
cd slave
npm install
cp config.example.yaml config.yaml
# 编辑 config.yaml：gatewayUrl / slaveId / repos[].cwd（必须为绝对路径白名单）

npm run build
npm start -- --config config.yaml

# 开发（tsx，免先 build）
npm run dev -- --config config.yaml

# 单测（cwd 白名单 / 拒绝 cloud）
npm test
```

配置示例见 `config.example.yaml`、密钥占位见 `.env.example`（均不含真实 Key）。

启动时会：

1. 加载 YAML（`-c` / `SLAVE_CONFIG` / 默认 `config.yaml`）
2. 校验 `repos[].cwd` 均为**绝对路径**白名单
3. 拒绝配置中的 `cloud` / Cloud Agent 默认项
4. 打结构化日志（脱敏）；本阶段不连 Gateway、不调 SDK

## 配置字段

| 字段 | 说明 |
|------|------|
| `gatewayUrl` | Slave 出站 WS，如 `ws://127.0.0.1:8080/v1/slave/ws` |
| `slaveId` | 与 Gateway / App 创建任务时一致 |
| `repos[]` | `id` / `name` / `cwd`（绝对路径） |
| `apiKeyEnv` | 存放 Cursor API Key 的**环境变量名**（默认 `CURSOR_API_KEY`） |
| `tokenEnv` | Gateway Bearer 的环境变量名（默认 `GATEWAY_TOKEN`，M04-P02 使用） |

## Roadmap

| Phase | 内容 |
|-------|------|
| **M04-P01** | 工程 + 配置加载 / cwd 白名单（当前） |
| M04-P02 | Gateway 出站客户端 |
| M04-P03 | `@cursor/sdk` Local Agent |
| M04-P04 | 取消 / 错误 / 安全默认 |

M03 协议联调仍可用 Gateway mock（无 SDK）：

```bash
cd ../gateway
go run ./cmd/mockslave -token <bearer> -id slave_devpc
```

协议：[../gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)  
设计：[doc/local-slave.md](../doc/local-slave.md)
