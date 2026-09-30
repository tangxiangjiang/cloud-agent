# Slave-Master：一工程一 Slave 与守护进程

**状态：设计（M11）**  
相关：现网多工程单进程见 [architecture.md](./architecture.md)、[deploy.md](./deploy.md)；可执行拆分见 [roadmaps/cloud-agent/milestones/M11-slave-master.md](./roadmaps/cloud-agent/milestones/M11-slave-master.md)。

---

## 问题

当前一台开发机上 **一个 Local Slave 进程** 配置 `projects[]` 多工程并行：

- DAG / Chat / Sync 共享同一进程与 SDK 运行时  
- 某一工程 Agent 卡死、OOM、未捕获异常 → **整机所有工程不可用**  
- 重启 Slave 影响面大，排查困难  

目标：**Slave : 工程 = 1 : 1**；用 **slave-master（守护进程）** 管多份配置与子进程生命周期；App 可看配置、启停、改配置。

---

## 角色

```
┌─────────────┐     HTTPS / WSS      ┌──────────────────┐
│ Flutter App │ ───────────────────► │  Gateway (Go)    │
│  (手机)     │ ◄─── 事件 / 列表 ─── │                  │
└─────────────┘                      └────────┬─────────┘
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    │ 出站 WS（控制面）         │ 出站 WS（数据面，每 Slave） │
                    ▼                         ▼                         ▼
           ┌────────────────┐      ┌──────────────┐          ┌──────────────┐
           │ slave-master   │      │ Slave A      │   …      │ Slave N      │
           │ （守护进程）    │──spawn─►│ 1 project    │          │ 1 project    │
           │ 配置 + 启停    │      │ → Gateway    │          │ → Gateway    │
           └────────────────┘      └──────────────┘          └──────────────┘
```

| 角色 | 职责 |
|------|------|
| **slave-master** | 本机守护：持有多份 Slave 配置；拉起 / 关闭 / 重启子进程；向 Gateway 上报配置与进程状态；接收 App 经 Gateway 下发的 CRUD / 启停 |
| **Slave（子进程）** | **仅绑定一个工程**（一个 `cwd` / `repoId`）；现有 Task / Workflow / Chat / Sync 逻辑基本不变；各自出站连 Gateway |
| **Gateway** | 控制面：Master 注册、配置镜像、启停指令转发；数据面：仍按 `slaveId` 找在线子 Slave |
| **App** | 看 Master 下的 Slave 配置与运行态；启停；增删改配置（**不**持 API Key；不传任意公网 cwd） |

**原则：**

- 控制面（配置 / 进程）与数据面（task / workflow / chat）分离  
- 子 Slave 故障互不影响；Master 挂了**不**强杀已在跑的子进程；重启后按规则收养，禁止重复 spawn 同 `slaveId`  
- `CURSOR_API_KEY` / `GATEWAY_TOKEN` 仍只在本机；App 只改「非密钥」配置字段，密钥用本机 env 名引用  

---

## 决议（M11 冻结）

下列条目为实现对齐基准；后续 milestone 可放宽，但 **M11 不得偏离**。

### 1. 鉴权：共享一对 Token

| 项 | 决议 |
|----|------|
| Master WS | 使用与现网配对相同的 Bearer（`tokenEnv`，默认 `GATEWAY_TOKEN`） |
| 子 Slave WS | **同一 token**；配置里不写每条 `tokenEnv` |
| 每 Slave 独立配对 | **非 M11**（后续可选） |

Gateway 吊销/轮换一对 token 即同时影响 Master 与全部子进程（可接受；简化实现）。

### 2. 权威源

| 数据 | 权威 | Gateway / App |
|------|------|----------------|
| Slave **配置**（id/name/cwd/index/enabled…） | **Master 本机配置文件** | Gateway 仅镜像最近成功上报；写配置必须等 Master `config.ok` |
| **进程态**（starting/running/stopping/stopped/error + pid） | **Master 进程表** | 经 `slaves.report` 镜像 |
| **数据面 online**（能否收 task） | **Gateway 子 Slave WS** | `GET /v1/slaves` 的 `online` |

App 写配置失败（`config.error` / 超时）→ 展示错误并以下次 `GET /masters/{id}`（或 Master 重报）为准，**不以**乐观本地缓存为准。

### 3. 三态模型（必须同时展示）

每个 Slave 行同时有：

| 字段 | 含义 | 来源 |
|------|------|------|
| `desired` | 配置意图：`enabled` true/false | Master 配置 |
| `process` | `stopped` \| `starting` \| `running` \| `stopping` \| `error` | Master |
| `gatewayOnline` | 子进程是否已向 Gateway 登记且连接有效 | Gateway |

常见对齐情况：

| process | gatewayOnline | UI 提示 |
|---------|---------------|---------|
| running | true | 正常，可进工程 |
| running | false | 「进程在、未连上 Gateway」→ 可 Restart / 查 lastError |
| stopped | false | 已停 |
| error | * | 显示 `lastError`；可 Restart |
| starting | false | 等待连上（超时进 error） |

**进工程入口**：要求 `process=running` **且** `gatewayOnline=true`（避免点进后发任务 503）。

### 4. 操作串行与幂等

- 所有控制指令带 `requestId`（App 或 Gateway 生成）；Master 回复带同一 id  
- **按 `slaveId` 串行队列**（同一 Slave 上 start/stop/upsert 不并行）  
- 幂等：`start` 已在 running → `ok`（no-op）；`stop` 已 stopped → `ok`  
- 全局：`maxConcurrentStarts`（默认 2），避免同时拉起过多 Node/SDK  
- 限流：Gateway 对 config/control HTTP 按用户限流（对齐 revise）  

### 5. 配额、开机与启动策略

```yaml
maxRunningSlaves: 4          # 同时 process=running 上限；超出 start → error
maxConcurrentStarts: 2       # 同时处于 starting 的上限
gracePeriodMs: 15000
```

| 项 | M11 决议 |
|----|----------|
| Master **冷启动** | 全部子进程保持 **stopped**；**不**因 `enabled=true` 自动 start |
| `enabled=false` | 禁止 Start（App/API 返回明确错误，如 `slave_disabled`） |
| `enabled=true` | 允许用户/API Start；不表示已在跑 |
| lazyStart / 空闲自动 stop | **不做**（后续可选） |
| Cursor Key | 多进程共用同一 `apiKeyEnv` 可能放大限流/账单；靠 `maxRunningSlaves` 约束，不做 Key 池 |

### 6. Stop 语义（与 Gateway cancel 衔接）

顺序冻结：

1. `process → stopping`  
2. **请 Gateway 取消该 `slaveId` 上进行中的 tasks**（现网 cancel 通道；尽力而为，可并行发）  
3. 等待 `gracePeriodMs`（默认 15s），期间子进程应自行收尾并断开 WS  
4. 仍存活 → **强制终止**（POSIX SIGKILL；Windows terminate / 结束进程树）  
5. **不**自动 approve、**不**改 Workflow 节点 status；半截 chat 走现网 Gateway 落库  
6. `process → stopped`；WS 断开后 `gatewayOnline=false`  

若 Gateway 不可达：跳过步骤 2，直接 grace → kill，并在 `lastError` 注明 `cancel_skipped_gateway_unreachable`（可选）。

### 7. Orphan 与收养（防重复 spawn）

- 状态目录固定在 **Master 工作目录**：`<masterRoot>/.local/slaves/<slaveId>/`（**不要**写进各 git 仓库）  
- 文件：`slave.pid`、可选 `spawnToken`、`config.yaml`、`logs/`  
- Master 启动 / 重连时 **reconcile**：  
  - pid 存活且 token 匹配 → **收养**，`process=running`，不二次 spawn  
  - pid 死或不匹配 → 清文件，保持 stopped（冷启动策略）  
- **禁止**同一 `slaveId` 双开；冲突 → **停新保旧** + 打日志  

### 8. `slaveId` / `repoId` / 标识

| 规则 | 说明 |
|------|------|
| 主工程迁移 | 保留旧 `slaveId`（见迁移节） |
| 其余工程 | `slave_<repoId>`，冲突加后缀 |
| `masterId` / `slaveId` | 字符集 `^[a-zA-Z0-9_][a-zA-Z0-9_-]{0,63}$` |
| 同 Master 下 **`project.id`（repoId）唯一** | 禁止两 Slave 绑同一 repoId |
| 同 Master 下 **`cwd` 唯一**（规范化后） | 禁止双进程抢同一工作区 |
| Gateway remap | M11 **不做** |

### 9. 配置变更边界

| 字段 | 规则 |
|------|------|
| `id`（slaveId） | **不可改**；只能删建 |
| `cwd` / `index` | 必须先 stop（或 upsert 时 Master 强制 stop）再改 |
| `cwd` 校验 | 绝对路径；存在；`allowedRoots`（若设）；拒 `..` |
| 删除 | stop → 删配置 → 清 `<masterRoot>/.local/slaves/<id>/` → 去镜像 |
| 本机手改 yaml | Master 启动或检测到变更后 **全量 `register` / report**；Gateway 镜像以本次上报为准 |

### 10. 控制面 HTTP：同步等待回执

| 项 | 决议 |
|----|------|
| CRUD / start / stop / restart | Gateway **同步等待** Master `*.ok` / `*.error` |
| 超时 | 默认 **30s** → HTTP `504`（`master_control_timeout`） |
| Master offline | `409` |
| WS 未送达 | `503` |

App **不**依赖 202+自建轮询（除非后续另开优化）。

### 11. App 信息架构

| 入口 | 用途 |
|------|------|
| **舰队管理（主）** | `/v1/masters` → 配置、三态、启停、CRUD |
| **工程入口** | 仅列出 `process=running` **且** `gatewayOnline=true` 的子 Slave（及其唯一 project） |
| `GET /v1/slaves` | 仍为数据面登记表；可含 offline 残留，**不作**舰队主 UI |

停掉的 Slave：舰队页显示 stopped；工程列表 **隐藏**（或灰显不可进）。

### 12. 模型目录（`models.report`）

- App「刷新模型」：Gateway 向 **任一 online 子 Slave** 发 `models.refresh`（与现网类似）  
- 多 Slave 上报：Gateway **取最新一次**合并/覆盖全局可选模型列表（不按工程拆分 Key）  
- 与一工程一 Slave **解耦**；不做每 Slave 独立模型商店（M11）  

### 13. 日志、排障与非目标探活

- 日志：`<masterRoot>/.local/slaves/<slaveId>/logs/`  
- report：`lastError`、可选 `logHint`（本地路径）  
- **不做** Agent/SDK 级 hang 检测（pid 在但卡死 → 用户 Restart）  
- M11 **不做** App 远程拉完整日志  

### 14. 错误码（控制面，建议稳定字符串）

| code | 含义 |
|------|------|
| `slave_disabled` | `enabled=false`，拒绝 start |
| `slave_limit` | 超过 `maxRunningSlaves` |
| `start_busy` | 超过 `maxConcurrentStarts` |
| `cwd_denied` | cwd 非法 / 不在 allowedRoots |
| `repo_conflict` | repoId 或 cwd 与已有 Slave 冲突 |
| `not_stopped` | 改 cwd 等要求先 stop |
| `id_immutable` | 试图修改 slaveId |
| `master_offline` | HTTP 409 |
| `master_control_timeout` | HTTP 504 |
| `cancel_skipped_gateway_unreachable` | stop 时未能请 Gateway cancel（写入 lastError 即可） |

---

## 配置模型

### Master 配置（例：`slave/master.config.yaml`）

```yaml
masterId: master_devpc
name: 台式机 Master
gatewayUrl: ws://127.0.0.1:8080/v1/master/ws
tokenEnv: GATEWAY_TOKEN          # Master + 全部子 Slave 共用

slaveCommand: ["node", "dist/index.js"]
slaveCwd: .
allowedRoots:                    # 可选；配置了则 cwd 必须在其下
  - E:/workspace
maxRunningSlaves: 4
maxConcurrentStarts: 2
gracePeriodMs: 15000

defaults:
  apiKeyEnv: CURSOR_API_KEY
  defaultModel: default
  autoModelId: default
  optimizeFor: cost

slaves:
  - id: slave_devpc              # 迁移：主工程保留旧 id
    name: cloud-agent
    enabled: true                # 允许 Start；冷启动仍为 stopped
    project:
      id: r_cloud_agent
      name: cloud-agent
      cwd: E:/workspace/cloud-agent
      index: ai/milestones.json
  - id: slave_r_flutter_chat
    name: flutter-chat-local
    enabled: true
    project:
      id: r_flutter_chat
      name: flutter-chat-local
      cwd: E:/workspace/flutter-chat-local
      index: ai/milestones.json
```

### 约束

| 规则 | 说明 |
|------|------|
| 一 Slave 一 `project` | 禁止子配置 `projects[]`；legacy 仅 `--legacy-multi-project` |
| `slaveId` / `masterId` | 见决议 §8 字符集；同 Gateway 下 `masterId` 唯一 |
| `project.id` / `cwd` | 同 Master 下均唯一 |
| `cwd` | 绝对路径 + 存在 + `allowedRoots`（若设） |
| 密钥 | 只写 env 名；禁止明文；共享 `tokenEnv` / `apiKeyEnv` |

### 子 Slave 运行时配置

Master 拉起时写入 **`<masterRoot>/.local/slaves/<id>/config.yaml`**（单 project）并记录 `slave.pid`。  
子进程 = 今日 `slave`，且 **`projects.length === 1`**；继承 Master 的 `tokenEnv` / `gatewayUrl`（数据面仍连 `/v1/slave/ws`）。

---

## 控制面协议（草案）

控制类消息均建议带 `requestId`。

### Master → Gateway

| 消息 | 说明 |
|------|------|
| `master.register` | `{masterId,name,slaves:[…]}` 全量配置 + 初始 runtime |
| `master.heartbeat` | 保活 |
| `master.slaves.report` | 单条或批量：`process`、`pid`、`lastError`、`desired` |
| `master.config.ok` / `master.config.error` | CRUD 结果（`requestId`） |
| `master.control.ok` / `master.control.error` | start/stop/restart 结果（`requestId`） |

### Gateway → Master

| 消息 | 说明 |
|------|------|
| `master.config.upsert` | `{requestId, slave}` |
| `master.config.delete` | `{requestId, slaveId}` |
| `master.control` | `{requestId, slaveId, action: start\|stop\|restart}` |
| `master.config.list` | 要求 Master 重报全量（对账） |

Gateway **持久化镜像**便于 Master 短暂掉线时 App 只读；**写配置 / 启停以 Master 成功回执为准**（见决议 §10）。

Master 可调用或由 Gateway 在 stop 流程中触发：**按 `slaveId` cancel 进行中 tasks**（数据面已有能力；控制面编排见决议 §6）。

### App → Gateway（HTTP，Bearer）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/masters` | Master 列表 |
| GET | `/v1/masters/{masterId}` | 配置 ∪ `process` ∪ `gatewayOnline` |
| POST | `/v1/masters/{masterId}/slaves` | 新增 → 等 `config.ok` |
| PUT/PATCH | `/v1/masters/{masterId}/slaves/{slaveId}` | 改配置（不可改 id） |
| DELETE | `/v1/masters/{masterId}/slaves/{slaveId}` | 先 stop 再删 |
| POST | `.../slaves/{slaveId}/start` | 等 `control.ok`（≤30s） |
| POST | `.../slaves/{slaveId}/stop` | cancel→grace→kill；等回执 |
| POST | `.../slaves/{slaveId}/restart` | stop+start |

`409` / `503` / `504` 见决议 §10、§14。  
数据面（tasks / workflows / chats / sync）**仍用子 `slaveId`**。

---

## App UX（建议）

| 区域 | 行为 |
|------|------|
| **舰队页（主）** | Master → Slave；三态 + `lastError`；启停 / CRUD |
| Slave 行 | 名称、cwd、process、gatewayOnline、lastError |
| 操作 | Start / Stop / Restart；编辑（id 只读）；删除（确认） |
| 添加 | id、name、cwd、index；等成功回执再刷新 GET |
| **工程入口** | 只展示 `running && gatewayOnline`；点进 Milestone / Chat / Sync |
| 模型管理 | 沿用全局列表；refresh 任意 online 子 Slave |

**不做：** App 填 API Key；绕过 Master 的 cwd；远程完整日志；以 `/v1/slaves` 作舰队主列表。

---

## 迁移

1. 读旧 `config.yaml` 的 `slaveId` + `projects[]`  
2. **第一条（或标记为 primary 的）project → `slaves[].id = 旧 slaveId`**  
3. 其余 → `slave_<repoId>`（去重）  
4. 写出 `master.config.yaml` + 映射摘要（markdown 或 json）  
5. 过渡期：旧单进程加 `--legacy-multi-project`（deprecated 日志）  
6. `GET /v1/slaves` 仍列子 Slave；Master 走 `/v1/masters`  

验收：主工程上迁移前创建的 Workflow，迁移后仍能对同一 `slaveId` start / review。

---

## 故障与隔离

| 场景 | 期望 |
|------|------|
| 子 Slave OOM / 崩溃 | 仅该工程；`process=error` + `lastError`；他工程不变 |
| Master 崩溃 | 子进程可继续；Master 重启 **收养** pid，禁止双开 |
| 重复 spawn 冲突 | 停新保旧 + error 日志 |
| 改 cwd | 强制先 stop |
| 删配置 | stop → 删条目 → 清本地目录 → 去镜像 |
| 超 `maxRunningSlaves` | start 失败，明确错误码/文案 |

---

## 安全

- Master / 子 Slave 均出站；Master **不对公网**暴露管理口（首版）  
- CRUD 仅经 Gateway Bearer；Master 校验 `cwd` / `allowedRoots`  
- 审计：`master.register` / `slave.config.*` / `slave.control.*`（无 Key / token）  
- 限流：配置变更与 start/stop  

---

## 非目标（M11）

- 多机 Master / K8s（多 Master 时仅要求 `masterId` 全局唯一）  
- 每 Slave 独立 Gateway 配对 token / Key 池  
- Gateway 侧 `slaveId` 自动 remap API  
- Master 代理全部 task 事件  
- App 远程拉完整日志 / 填 API Key  
- 冷启动自动拉起、`enabled` 即 running、空闲自动 stop、进工程 lazy-start  
- Agent/SDK hang 探活（卡死靠用户 Restart）  
- 每工程独立模型商店  

---

## 实现分期（对应 M11 phases）

| Phase | 内容 |
|-------|------|
| **P1** | 设计落地 + 配置 schema + 契约（含本文决议） |
| **P2** | Master：冷启动全停、串行队列、stop 衔接 cancel、收养、配额 |
| **P3** | 子 Slave 单工程；迁移（主工程保留旧 slaveId）；repoId/cwd 唯一 |
| **P4** | Gateway Master WS + `/v1/masters*`；同步等 ok；合并 gatewayOnline；stop 时 cancel |
| **P5** | App：舰队主入口 + 工程入口过滤；三态；启停 CRUD |
| **P6** | 联调、审计、deploy（含 Master 自启可选说明）、废弃旧 multi-project |

Roadmap：[M11](./roadmaps/cloud-agent/milestones/M11-slave-master.md)。
