# 本机部署与联调（Gateway + Slave + App）

**Local only**：Agent 仅 `@cursor/sdk` **Local** runtime（`local.cwd`）。**禁止** Cloud Agent / 云端整仓。  
密钥只放环境变量，**不要**提交进 git。

## 组件

| 进程 | 目录 | 作用 |
|------|------|------|
| Gateway | `gateway/` | HTTP + App WS + Slave / Master 出站 WS |
| slave-master | `slave/` | 守护：配置与启停多个 1:1 子 Slave（M11） |
| Slave | `slave/` | Local Agent + DAG（一工程一进程） |
| App | `app/` | 舰队管理、工程入口、工作流、Chat |

一键联调：`python run.py up`（默认 Master；`--legacy-slave` 为旧单进程）。

## 环境变量

| 变量 | 谁 | 说明 |
|------|-----|------|
| `GATEWAY_PAIR_CODE` | Gateway | 可选；空则启动时生成 |
| `GATEWAY_ADDR` | Gateway | 默认 `:8080` |
| `GATEWAY_AUDIT_LOG` | Gateway | 可选审计 JSONL 路径 |
| `GATEWAY_TOKEN` | Slave | `POST /v1/auth/pair` 拿到的 Bearer |
| `CURSOR_API_KEY` | Slave | Cursor API Key（**绝不**进 App / 配置明文） |
| `SLAVE_CONFIG` | Slave | 可选配置路径 |

## 启动顺序

### 1. Gateway

```bash
cd gateway
go run . -pair-code ABCD-EFGH
# 日志: pair code: ABCD-EFGH
```

### 2. 配对拿 Token

```bash
curl -s -X POST http://127.0.0.1:8080/v1/auth/pair \
  -H "Content-Type: application/json" \
  -d "{\"pairCode\":\"ABCD-EFGH\"}"
# → { "token": "...", "expiresAt": "..." }
```

### 3. Slave（单工程）或 slave-master（推荐多工程）

**默认（M11）：一个 Slave 进程只绑一个 `projects[]` 条目。** 多工程请用 Master。

#### 3a. 单工程直接启动（仍可用）

编辑 `slave/config.yaml`（从 `config.example.yaml` 复制）：恰好 **一个** project；`cwd` 为绝对路径；`slaveId` 与创建工作流时一致。

```bash
cd slave
# Windows PowerShell:
$env:GATEWAY_TOKEN="<token>"
$env:CURSOR_API_KEY="<cursor-key>"
npm start
# 或 stub 联调（不调 SDK）: npm run dev -- --stub
# 旧多工程逃逸（deprecated）: npm start -- --legacy-multi-project
```

#### 3b. slave-master（多工程 / 舰队）

1. 从旧配置迁移（主工程保留原 `slaveId`）：

```bash
cd slave
npm run master -- migrate --from config.yaml --to master.config.yaml
```

2. 编辑 `master.config.example.yaml` → `master.config.yaml`：核对 `allowedRoots`、`maxRunningSlaves`、`defaults.slaveGatewayUrl`（子进程数据面）、`gatewayUrl`（Master 控制面 `/v1/master/ws`）。

3. **冷启动不拉起子进程**（`enabled=true` 只表示允许 Start）。本机守护：

```bash
$env:GATEWAY_TOKEN="<token>"
$env:CURSOR_API_KEY="<cursor-key>"
npm run master -- serve
```

4. 本地 CLI（无需 Gateway）：`npm run master -- status|start|stop|restart <slaveId>`。  
   状态目录：`<masterRoot>/.local/slaves/<slaveId>/`（勿放进业务仓库）。

5. App **舰队**页 Start 子 Slave；**工程**页仅显示 `process=running` 且 Gateway 数据面 online 的条目。

共享同一 `CURSOR_API_KEY` / `GATEWAY_TOKEN`；用 `maxRunningSlaves` 约束同时跑的 Agent 进程数（账单/限流）。

可选：用 OS 服务/计划任务自启 `npm run master -- serve`，**不要**配置成自动 Start 全部子进程。

### 4. App

```bash
cd app
flutter run
```

配对页：

- Gateway URL：模拟器 Android `http://10.0.2.2:8080`；本机 / iOS sim `http://127.0.0.1:8080`
- Pair code：`ABCD-EFGH`

### 5. 创建样例工作流并 Start

PowerShell 示例（`examples/sample-dag.json`）：

```powershell
$TOKEN = "<token>"
$bundle = Get-Content examples/sample-dag.json -Raw | ConvertFrom-Json
$body = @{
  bundleId = $bundle.id
  bundleRef = "examples/sample-dag.json"
  slaveId = $bundle.preferredSlaveId
  repoId = $bundle.repoId
  progressDoc = $bundle.progressDoc
  nodes = $bundle.nodes
} | ConvertTo-Json -Depth 10
$wf = Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/v1/workflows `
  -Headers @{ Authorization = "Bearer $TOKEN" } -ContentType "application/json" -Body $body
Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v1/workflows/$($wf.id)/start" `
  -Headers @{ Authorization = "Bearer $TOKEN" }
```

App：**Workflows** → 详情 → **Start** → 节点 `awaiting_review` → Diff / Review（revise 或 approve）。  
Approve 后下游变为 `ready`，**默认不会自动开跑**；需再点 Continue / 节点 Start，或创建时给节点设 `policy.autoStartNext: true`（见 [node-policy.md](./node-policy.md)）。

## 常见失败

| 现象 | 排查 |
|------|------|
| pair 401 | pair code 不一致；注意大小写与连字符 |
| pair / revise 429 | 限流：见 [gateway/docs/audit.md](../gateway/docs/audit.md) |
| Slave 无任务 | `slaveId` / `repoId` 与配置不一致；Slave 未 online |
| Agent 失败 | 缺 `CURSOR_API_KEY`；或用 `--stub` 先通协议 |
| App 连不上 | 模拟器用 `10.0.2.2`；真机用电脑局域网 IP；允许 cleartext HTTP（debug） |
| Diff 404 | 节点尚未上传 diff；等 Agent 进 `awaiting_review` |
| approve 后无进度勾选 | `progressDoc` 须相对仓库 cwd 且 basename=`progress.md` |

## 安全再强调

- App **不**持有 `CURSOR_API_KEY`
- Token 在 App 用 `flutter_secure_storage`；日志仅脱敏尾缀
- 正式环境用 HTTPS/WSS；本机 debug 可用 HTTP
