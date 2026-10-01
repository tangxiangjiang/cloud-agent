# 节点执行策略（模型 / 自动通过 / 自动续跑）

**状态：节点策略（续跑门闩 / App 控件 / 自动通过 / 创建默认）已落地（M10-P01～P04）**  
入口：App **工作流详情**里每个任务（节点）行上的控件，形态对齐设置页的 **下拉 + Switch**（默认关）。

解决：当前 Approve 后 Gateway 会再次 `workflow.assign`，**自动开跑下一 ready 节点**；部分场景希望人审完停住，手动点下一个再跑。同时支持每节点选模型、可选自动通过。

相关：[workflow.md](./workflow.md)、[project-chat.md](./project-chat.md)（Auto 模型语义一致）。

---

## 每节点控件（App）

| 控件 | 类型 | 默认 | 含义 |
|------|------|------|------|
| **模型** | 下拉 | **Auto** | 本节点 Local Agent 模型；`auto` → Slave 按账号解析（Pro=`default`，团队 Router=`auto-smart`） |
| **自动续跑** | Switch | **关** | 合并「自动通过 + 自动开始下个」：跑完自动 Approve，并自动开跑下游 ready 节点 |

示意（任务行右侧 / 展开区）：

```text
M01-P01  消息模型 …     [Auto ▾]  自动续跑 ○──
M01-P02  气泡 …         [Auto ▾]  自动续跑 ●──  (开)
```

- Switch **默认关闭**（安全默认：人工审核 + 人工点续跑）。  
- 可在 **创建 Workflow 时**带上每节点策略；也可在详情页修改（PATCH），对**尚未 running** 的节点立即生效；已 running 的 model 以下发时快照为准。
- 协议仍存 `autoApprove` / `autoStartNext` 两字段；App 开关会**同时写入**二者。

---

## 行为

| 自动续跑 | 行为 |
|----------|------|
| 关 | Agent 跑完 → `awaiting_review` → 人 Approve → 下游变 `ready`，**不**自动 assign；用户点「开始该节点」或全局「继续」 |
| 开 | 跑完后自动 Approve（写 progress + commit）→ 自动 `AssignWorkflow` 续跑下游（全自动串行；仍审计） |

**Reject** 不受「自动续跑」影响；驳回后不自动续跑。

---

## 数据模型

节点上增加策略字段（创建 Workflow / PATCH 节点）：

```json
{
  "id": "M01-P01",
  "model": "auto",
  "policy": {
    "autoApprove": false,
    "autoStartNext": false
  }
}
```

| 字段 | 类型 | 默认 |
|------|------|------|
| `model` | string | `"auto"` |
| `policy.autoApprove` | bool | `false` |
| `policy.autoStartNext` | bool | `false` |

兼容：旧客户端不传 `policy` → 两开关视为 `false`。  
**迁移注意：** 现网 Approve 后总会 `AssignWorkflow`；实现本功能后默认改为 **仅当 `autoStartNext==true` 才自动 assign**（行为变化，属有意收紧）。

可选 Workflow 级默认（创建时）：

```json
{
  "defaultPolicy": {
    "autoApprove": false,
    "autoStartNext": false
  },
  "defaultModel": "auto"
}
```

节点未写的 `model` / `policy` 继承上述默认；节点显式字段优先生效。  
App「执行 plan」页可批量设「本 Milestone 默认」（两开关默认关，可关闭）。

---

## Gateway / Slave

### Approve 路径（改动点）

今日：`review approve` → 节点 approved → `RecomputeReady` → **`AssignWorkflow`**。  

改为：

1. 始终：approved + RecomputeReady + 通知 Slave 写 progress（现逻辑）  
2. **仅当**刚通过的节点 `policy.autoStartNext == true`（或「继续」API）时再 `AssignWorkflow`  
3. 否则 Workflow 保持 `running`/`pending` 视状态机约定，下游为 `ready`，等：  
   - `POST /v1/workflows/{id}/nodes/{nodeId}/start`，或  
   - `POST /v1/workflows/{id}/continue`（调度所有 ready / 下一个）

### 自动通过

节点进入 `awaiting_review` 时（Slave `PATCH` status）：

- 若 `policy.autoApprove==true`：Gateway **立刻**走与人工 Approve **相同**管道：节点 → `approved` → `RecomputeReady` → `workflow.review`（`autoApprove: true`）→ Slave 写 progress + git commit  
- 若同时 `autoStartNext`：再 `AssignWorkflow`；否则下游停在 `ready`  
- 审计：`workflow.review` meta 含 `autoApprove=true|false`（可区分自动 / 人工）  
- **Reject** 仅经 `POST .../review`；不受 autoApprove 影响  
- 空 diff 也允许自动通过（首版），但审计必记  

安全：App 改 `autoApprove` 需 online 会话；审计可查。

### 模型

`task.assign` / 节点执行时带 `model`；`auto` 由 Slave 按账号目录解析。与 Chat 共用模型列表 API（若已有）。

---

## App UI

**工作流详情**每个节点 `ListTile` / 展开：

1. 模型 `DropdownButton`（Auto + 列表）  
2. `Switch` 自动通过  
3. `Switch` 自动开始下个任务  
4. 当节点 `ready` 且未自动开跑：显示 **开始** 按钮  

修改策略：`PATCH /v1/workflows/{id}/nodes/{nodeId}`  

```json
{
  "model": "composer-2.5",
  "policy": { "autoApprove": false, "autoStartNext": true }
}
```

创建 Milestone Workflow 时：App「执行 plan」页可设 `defaultModel` / `defaultPolicy`（默认 Auto + 两开关关）；Gateway 写入各未显式指定的节点。

---

## API 增量草案

| 方法 | 路径 | 说明 |
|------|------|------|
| PATCH | `/v1/workflows/{id}/nodes/{nodeId}` | 扩展写 `model` / `policy`（已有 PATCH 可扩展） |
| POST | `/v1/workflows/{id}/nodes/{nodeId}/start` | 仅 start 该 ready 节点（下发 assign 聚焦或全量由 Slave 只跑 ready） |
| POST | `/v1/workflows/{id}/continue` | 按当前策略续跑（至少启动所有 ready） |

创建：`POST /v1/workflows` 可带 `defaultModel`、`defaultPolicy`；节点未写字段继承之。  

创建节点请求体亦可直接带 `model` / `policy`（见上）。

---

## 实现分期

| Phase | 内容 |
|-------|------|
| **P1** | Gateway：Approve 后默认**不**自动 assign；`autoStartNext` / `continue` / 节点 start |
| **P2** | App 节点行：两 Switch + 模型下拉；PATCH 策略 |
| **P3** | `autoApprove` 全链路 + 审计标记 |
| **P4** | 执行 plan 页批量默认策略 |

Roadmap：[M10](./roadmaps/cloud-agent/milestones/M10-node-policy.md)。

---

## 非目标

- 按文件路径的细粒度权限  
- 自动通过且跳过 git commit / progress（Approve 管道不可拆，除非显式「仅标记」——不做）  
- 改变串行 DAG 拓扑（仍一次一个 ready，除非另开并行里程碑）  
