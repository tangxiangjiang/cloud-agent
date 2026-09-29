# 工程 AI 对话（Project Chat）

**状态：Gateway Chat→Task 桥接已落地（M09-P01）；App UI / Ask·Plan 完善 / 历史见后续 phase**  
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
| **Auto**（默认） | 不传具体 model，或传 `auto`；Slave 用 `defaultModel` / 服务端推荐映射 |
| 具体 id | 如 `composer-2.5`；来自 Gateway 下发的 **允许列表**（避免 App 写死过时列表） |

`GET /v1/models`（或随 `GET /v1/slaves` 附带）返回：

```json
{
  "default": "auto",
  "models": [
    { "id": "auto", "label": "Auto" },
    { "id": "composer-2.5", "label": "Composer 2.5" }
  ]
}
```

首版列表可由 Gateway 配置 / Slave register 上报合并；未知 id 时 Slave 回退 `defaultModel` 并在事件里注明。

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

- `GET /v1/chats?slaveId=&repoId=`  
- `GET /v1/chats/{id}`（含 messages 摘要）  
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

`model: auto` → Slave 解析为配置的 `defaultModel`（或后续路由策略）。

---

## App UI（建议）

| 区域 | 行为 |
|------|------|
| AppBar | 工程名；模式 Segmented：Agent / Ask / Plan；模型下拉（默认 Auto） |
| 消息区 | 用户气泡 + assistant 流式；tool 可折叠一行 |
| 底栏 | 多行输入、发送、停止（running 时） |
| 会话 | 右上「新对话」；可选历史抽屉（P2） |

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
| **P2** | Ask / Plan 前缀与 UI 切换；模型列表 API；停止 / 新会话 |
| **P3** | 会话历史持久化（SQLite）；多会话列表 |
| **P4**（可选） | 「提交为审核」→ 生成 diff 进 awaiting_review |

Roadmap：[M09](./roadmaps/cloud-agent/milestones/M09-project-chat.md)。

---

## 非目标（首版）

- 复刻 Cursor 全量 Composer / Tab / Inline Edit  
- App 内选文件@、完整 MCP 配置  
- Cloud Agent  
- 手机本地跑模型  
