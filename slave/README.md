# slave

Node.js / TypeScript **Local Slave**：出站连接 Go Gateway，用官方 `@cursor/sdk` **Local runtime** 执行任务（禁止 Cloud Agent / `cloud`）。

## 要求

| 依赖 | 版本 / 说明 |
|------|-------------|
| **Node.js** | **>= 22.13**（`@cursor/sdk` engines） |
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
# 编辑 projects[].cwd 为绝对路径；index 指向工程内 milestones.json

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
$env:CURSOR_API_KEY = "<cursor-api-key>"
npm run smoke-local
```

成功时日志含 `assistant.delta` / `tool.*`，并以 `status: finished` 结束。启动失败 exit 1；run 失败 exit 2。

## 行为

1. 校验配置与 `projects[].cwd` 绝对路径白名单；拒绝配置中的 `cloud` / 明文 `apiKey`
2. 启动时检查各工程是否为 git 仓库；若无（或无 HEAD）则自动 `git init` + 初始 commit（Diff 基线需要）
3. 出站 WS：`auth` → `register`（含 `projects` + milestones）→ `heartbeat`；断线重连
4. `task.assign` → 仅用 `repoId` 查白名单 cwd → Local Agent → stream 映射 → **必** `wait` → dispose
5. `workflow.assign`（`POST /workflows/{id}/start`）→ **串行**取 `ready` 节点 → 创建 task → 执行 → 节点进 `awaiting_review`（**绝不**直接 approved）；下游须依赖节点 `approved` 后才 `ready`
6. `workflow.revise` → follow-up / 再跑 → 刷新 diff → 再 `awaiting_review`（不写 progress）
7. `workflow.review` approve → **仅此时**更新 `progressDoc`（须为相对路径且 basename=`progress.md`）；reject 不写；approve 后 Gateway 再 `workflow.assign` 续跑下游
8. `project.sync` → 白名单 cwd 内**只读**采集 git / milestones / progress → 可选 Local Agent 短摘要（失败回退规则摘要；可用 `syncAiSummary: false` 或 `SYNC_AI_SUMMARY=0` 关闭）→ `POST /v1/project-sync`（失败则 WS `project.sync.result`）；不写工作区、不 `git commit`
9. `task.cancel` → `run.cancel()`（若 `supports`）；否则记原因并停止转发后续 stream（含 tool）
10. 错误：`phase: policy|startup|run`；日志脱敏

样例两节点 DAG：`fixtures/dag-two-node.json`（亦见 `ai/bundles/m05-p02-two-node.json`）。

Diff：[docs/diff-baseline.md](./docs/diff-baseline.md) · Approve 写进度：[docs/progress-approve.md](./docs/progress-approve.md)。

## 安全注意事项

- **API Key / Bearer 只放环境变量**（`apiKeyEnv` / `tokenEnv` 只写变量名）。配置文件禁止出现 `apiKey` 字面量；日志会对 `apiKey`/`token` 等字段与长 opaque 串脱敏。
- **cwd 白名单**：执行目录只来自本机 `projects[].cwd`，通过任务的 `repoId` 查找。**忽略/拒绝**任务里携带的任意 `cwd` 字符串，防止 App 指到白名单外路径。
- **仅 Local runtime**：`Agent.create` 只传 `local: { cwd, settingSources: [] }`，禁止 `cloud`，默认不加载 `settingSources: "all"`。
- **不对公网开 Slave HTTP**；Slave 只出站连 Gateway。
- 取消尽力而为：SDK 支持则 `run.cancel()`；不支持则停止向 Gateway 转发新的 tool/assistant 事件，仍 `wait()` 收尾。
- Key 泄露后：在 Cursor Dashboard 吊销并轮换；勿把 `.env` / `config.yaml` 提交进 git（已 gitignore）。

## 配置字段

| 字段 | 说明 |
|------|------|
| `gatewayUrl` | Slave 出站 WS |
| `slaveId` | 与创建任务一致 |
| `projects[]` | `id` / `name` / `cwd`（绝对路径）/ `index?`（相对 cwd 的 milestone JSON） |
| `repos[]` | 兼容旧字段；等同无 `index` 的 `projects` |
| `apiKeyEnv` | Cursor API Key 环境变量名（默认 `CURSOR_API_KEY`） |
| `tokenEnv` | Gateway Bearer 环境变量名（默认 `GATEWAY_TOKEN`） |
| `defaultModel` | task 未带 model 时使用（默认 `composer-2.5`） |

工程 milestone 索引约定见仓库根 [`ai/milestones.md`](../ai/milestones.md)。

## Roadmap

| Phase | 内容 |
|-------|------|
| M04-P01 | 工程 + 配置 / cwd 白名单 |
| M04-P02 | Gateway 出站客户端 |
| M04-P03 | Local Agent create/send/stream/wait |
| M04-P04 | 取消 / 错误 / 安全默认 |
| M05-P02 | 串行 DAG + `awaiting_review` 闸门 |
| M05-P03 | 只读 NodeDiff + git 基线 |
| M05-P04 | Revise follow-up 循环 |
| **M05-P05** | Approve 写 `progressDoc` + 解锁下游（当前） |

协议：[../gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)  
设计：[doc/local-slave.md](../doc/local-slave.md) · [doc/architecture.md](../doc/architecture.md)
