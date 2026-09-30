# 工程 AI 对话（Project Chat）

**状态：工程对话（含 Ask/Plan、模型列表、会话历史）已落地（M09-P01～P04）**  
入口：App **Slave → 工程 → 对话**（与 Milestone / 同步并列）。  
体验对齐 Cursor 对话：**Agent / Ask / Plan** + **模型选择**（默认 **Auto**）。

约束（不变）：

- App **不持有** `CURSOR_API_KEY`，不直连 Slave  
- 执行只在工程白名单 `cwd`（Local Agent）  
- 流式输出走现有 App WS（`assistant.delta` / tool / status）  

相关：自由任务雏形见 [api-outline.md](./api-outline.md) `POST /v1/tasks`；审核闸门见 [workflow.md](./workflow.md)。本功能是**会话式**自由任务，不替代 Milestone DAG。

---

## 用户路径

```text
Slave → 工程
         ├─ Milestone …
         ├─ 同步 …
         └─ 对话  →  Chat 页
                      ├─ 模式：Agent | Ask | Plan
                      ├─ 模型：Auto | 具体 model id
                      ├─ 消息列表（流式）
                      └─ 输入框 / 停止 / 新会话
```

---

## 三种模式（语义）

| 模式 | 意图 | Slave / SDK 侧建议 |
|------|------|-------------------|
| **Agent** | 可改代码、跑工具（默认「干活」） | 正常 Local Agent；工具可用 |
| **Ask** | 只读问答、解释代码 | 系统前缀强调 **read-only**；尽量不用写文件/改仓工具（SDK 若支持则关写；否则强 prompt + 审计） |
| **Plan** | 出方案 / 步骤，少改或不改代码 | 系统前缀要求输出结构化计划；默认不落地改文件，除非用户下一条明确「按计划执行」（可转 Agent） |

模式存在 **会话级默认** + **单条可覆盖**（与 Cursor 类似：顶栏切换影响后续发送）。

---

## 模型选择

| UI | 含义 |
|----|------|
| **Auto**（默认） | 传 `auto`；Slave 按账号目录解析：Pro 多为 `default`；Teams Router 可为 `auto-smart` + `optimize_for`（见 `autoModelId` / `optimizeFor`） |
| 具体 id | 如 `composer-2.5`；来自 Gateway 下发的 **允许列表**（避免 App 写死过时列表） |

`GET /v1/models`（Bearer）返回 **App 选择列表**（可在 App「模型列表」页增删；首启种子来自 `GATEWAY_MODELS` / 内置 Auto+常用 id）。  
`GET /v1/models/manage` 另含 Slave 上报的 Cursor `available`；Pro 目录可能只有 `default`，可用手动添加 `composer-2.5` 等 id。

```json
{
  "default": "auto",
  "models": [
    { "id": "auto", "label": "Auto" },
    { "id": "composer-2.5", "label": "Composer 2.5" }
  ]
}
```

未知 id 时仍原样传给 SDK；空 model 回退 Slave `defaultModel`。Auto 按账号解析（Pro 多为 `default`）。

### Ask / Plan 系统前缀

Gateway `BuildPrompt` 与 Slave `applyChatModePrefix` 双端注入（Slave 若发现已有 `[Mode: Ask` / `[Mode: Plan` 则不重复）：

- **Ask**：`[Mode: Ask — READ-ONLY. …]` — 禁止写文件 / 变仓库  
- **Plan**：`[Mode: Plan — Produce a structured plan…]` — 默认不改仓，除非用户明确执行  
- **Agent**：不加前缀

---

## 数据模型

### ChatSession（Gateway）

| 字段 | 说明 |
|------|------|
| `id` | `chat_…` |
| `slaveId` / `repoId` | 绑定工程 |
| `mode` | `agent` \| `ask` \| `plan` |
| `model` | `auto` 或 model id |
| `status` | `idle` \| `running` \| `error` |
| `createdAt` / `updatedAt` | |
| `messages[]` | 可选持久化摘要；大体量 delta 仍以 task events 为准 |

### 与 Task 的关系

推荐：**一次用户发送 = 一个 Task**（或 follow-up），挂在 `chatId` 下：

```text
ChatSession
  └─ turns[] → taskId  → 现有 task.event 流
```

这样复用：排队、取消、`GET /tasks/{id}/events`、App WS 订阅。  
会话历史：Gateway 存 role/content 文本（截断）；不存完整 tool payload。

### 消息截断策略（防爆库）

落库只保留 **摘要文本**（`role` / `content` / 可选 `taskId`），**禁止**持久化 tool 原始 payload、密钥、cwd。

| 限制 | 值 | 行为 |
|------|-----|------|
| 单条 content | ≤ **4000** 字符（rune） | 超长截断并加 `…` |
| 每会话 messages | ≤ **80** 条 | 丢弃最旧，保留最近 |
| 列表 API | 不含 messages 正文 | 返回 `preview`（最近 user 摘要）+ `messageCount` |

Assistant 摘要由 App 在 turn 结束后 `POST /v1/chats/{id}/assistant` 上报；流式 delta 仍只走 Task events。

持久化：`GATEWAY_STATE_FILE` SQLite 表 `chat_sessions`（无 state-file 时进程内 Memory，重启丢失）。

---

## API / WS 草案

### 创建会话

`POST /v1/chats`

```json
{
  "slaveId": "slave_devpc",
  "repoId": "r_flutter_chat",
  "mode": "agent",
  "model": "auto"
}
```

→ `201 { "id": "chat_…", … }`

### 发消息

`POST /v1/chats/{id}/messages`

```json
{
  "text": "给登录页加个 loading",
  "mode": "agent",
  "model": "auto"
}
```

→ `202 { "taskId": "tsk_…", "chatId": "chat_…" }`  
Gateway 组 prompt（注入 mode 前缀）→ 现有 `task.assign`。

### 列表 / 历史

- `GET /v1/chats?slaveId=&repoId=` → `{chats:[{id,preview,messageCount,…}]}`（无 messages 正文）  
- `GET /v1/chats/{id}` → 含截断后的 `messages[]`  
- `POST /v1/chats/{id}/assistant` → `{taskId?,content}` 写入 assistant 摘要  
- 实时：`GET /v1/ws` 订阅 `taskId`（与现网一致）

### 停止

`POST /v1/tasks/{taskId}/cancel`（已有）或 `POST /v1/chats/{id}/stop`

### Slave

沿用 Local Agent；`task` 增加可选字段：

```json
{
  "chatId": "chat_…",
  "mode": "ask",
  "model": "auto",
  "prompt": "…"
}
```

`model: auto` → Slave 启动时 `Cursor.models.list()`，优先用目录里真实存在的 id（Pro 通常只有 `default`；团队版才有 `auto-smart`）。空 model → `defaultModel`（默认 `default`）。  
硬编码 `auto-smart` 在 Pro 上无效/浪费额度；团队 Router 才设 `autoModelId: auto-smart`，且优先 `optimizeFor: cost`。

---

## App UI（建议）

| 区域 | 行为 |
|------|------|
| AppBar | 工程名；模式 Segmented：Agent / Ask / Plan；模型下拉（默认 Auto） |
| 消息区 | 用户气泡 + assistant 流式；tool 可折叠一行 |
| 底栏 | 多行输入、发送、停止（running 时） |
| 会话 | 右上「历史」底栏列表 +「新对话」 |

**与审核关系：** Agent 模式改代码**不强制**走 `awaiting_review`（自由对话）；若需「对话里改完也要审」，P3 再加「提交审核」生成临时单节点 Workflow（非首版）。

---

## 安全

- `repoId` 白名单；cwd 仅 Slave 解析  
- Ask/Plan：prompt +（能力允许时）工具策略双保险  
- 审计：`chat.create` / `chat.message` / cancel  
- 限流：每用户/设备发送速率（对齐 revise）  
- 不在 App 日志打印 token  

---

## 实现分期

| Phase | 内容 |
|-------|------|
| **P1** | `POST /chats` + `/messages` 映射 Task；App 基础对话页（仅 Agent + Auto/单模型） |
| **P2** | App 对话 UI（流式 / 停止 / 新会话） |
| **P3** | Ask / Plan 前缀与 UI 切换；`GET /v1/models` |
| **P4** | 会话历史持久化（SQLite）；多会话列表 |
| **P5**（可选） | 「提交为审核」→ 生成 diff 进 awaiting_review |

Roadmap：[M09](./roadmaps/cloud-agent/milestones/M09-project-chat.md)。

---

## 非目标（首版）

- 复刻 Cursor 全量 Composer / Tab / Inline Edit  
- App 内选文件@、完整 MCP 配置  
- Cloud Agent  
- 手机本地跑模型  
