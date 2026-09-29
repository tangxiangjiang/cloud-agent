# 工作流：从人工设计到 AI 可执行 DAG

本项目不只「手机发一句 prompt」。完整闭环是：

**IDE（人+AI）粗设计与拆分 → DAG → Slave 编码 → App 审核 → Slave 更新进度文档 → 下一节点。**

工程内已具备较全的 AI 开发规则、文档与源码；**设计 / 拆 phase / 编 DAG / 失败策略** 都在 Cursor IDE 由人+AI 完成（通常比写代码更快）。  
**手机不做设计**，只做：触发执行、跟踪进度、**审核编码结果**。

## 两段分工

| 阶段 | 谁主导 | 在哪 | 产出 |
|------|--------|------|------|
| **设计段** | 人 + AI（规则/文档/源码加持） | Cursor IDE | 架构、实施文档、milestones、phases、DAG、失败策略 |
| **执行段** | Slave 编码 + **App 人工闸门** | 本机 Slave + 手机 | Agent 改代码 → 人审核 → 通过后才推进进度文档 |

粒度变化：**大（架构）→ 中（milestone）→ 小（phase）→ 机器可读（DAG node）**。

## 设计段流水线（IDE 内，人工）

```
架构设计 (doc/architecture 等)
        ↓
实施文档 / 方案细节
        ↓
roadmaps/ 下生成一个个 Milestone
        ↓
每个 Milestone 切成若干 Phase 文档
（单次 Agent 会话可执行的粒度）
```

约定（建议目录，实现时可微调）：

```
roadmaps/
  <project-or-epic>/
    README.md                 # epic 目标与 milestone 索引
    milestones/
      M01-xxx.md              # 里程碑：目标、验收、依赖
    phases/
      M01-P01-xxx.md          # 单次 AI 可执行：范围、约束、完成定义
      M01-P02-xxx.md
workflow/
  <run-id>.dag.json           # 由工具从 phases 编译出的可执行 DAG（示例路径）
```

### Milestone 文档应有什么

- 目标与非目标  
- 依赖的上游 milestone  
- 验收标准  
- 下属 phase 列表（或生成后回填）

### Phase 文档应有什么（AI 可执行粒度）

一次 Local Agent run（或短多轮）能吃完，避免「做一个完整子系统」这种过大包：

- **目标**：改什么、做成什么样  
- **范围**：允许动的路径 / 禁止动的路径  
- **上下文**：必读文档或符号（链接到 `doc/`、接口草案）  
- **完成定义（DoD）**：可检查条件（测试过、接口对齐、文档已更新等）  
- **禁止事项**：例如不得引入 Cloud Agent、不得改密钥文件  

Phase 仍是给人看的 Markdown；**还不是** Slave 直接加载的结构。

## 编译段：Markdown → AI 可执行数据结构（DAG）

用 **AI + 工具**（IDE 内 Agent、脚本、或后续专用 compiler）把 milestone/phases **编译**成机器可读工作流。

倾向形态：**DAG（有向无环图）**。

| 概念 | 含义 |
|------|------|
| Node | 一个可调度单元，通常对应一个 Phase（或 phase 内一步） |
| Edge | 依赖：`A → B` 表示 B 须等 A 成功 |
| 并行 | 无互相依赖的节点可并行（首版可串行化，DAG 仍保留并行信息） |
| 状态 | `pending / ready / running / awaiting_review / approved / rejected / failed / cancelled / skipped` |

> Agent 跑完 **不等于** 节点成功。须经 App **审核通过（approved）** 后，Slave 才更新进度文档，下游节点才变 `ready`。  
> 下游是否**自动开跑**、是否**自动通过**审核，由节点策略控制（默认均关）：见 [node-policy.md](./node-policy.md)。

### DAG 节点字段（草案）

```json
{
  "id": "wf_20260928_01",
  "version": 1,
  "repoId": "r_cloud_agent",
  "slaveId": "slave_devpc",
  "nodes": [
    {
      "id": "M01-P01",
      "phaseRef": "roadmaps/.../phases/M01-P01-xxx.md",
      "title": "搭建 Gateway HTTP 骨架",
      "promptTemplate": "按 phase 文档执行……",
      "cwd": "E:/workspace/cloud-agent",
      "model": "composer-2.5",
      "dependsOn": [],
      "dodChecks": ["go test ./...", "文档已更新"]
    },
    {
      "id": "M01-P02",
      "phaseRef": "roadmaps/.../phases/M01-P02-xxx.md",
      "dependsOn": ["M01-P01"],
      "dodChecks": []
    }
  ]
}
```

编译器职责（工具，可后做）：

1. 解析 milestone / phase Markdown  
2. 校验依赖无环、phase 粒度提示（可选）  
3. 生成 `promptTemplate`（注入 phase 正文或路径，供 Slave 拼最终 prompt）  
4. 写出 `.dag.json`（或等价格式），校验 schema  

**人工仍可改 DAG**；App/Slave 以加载到的 **WorkflowRun 快照**为准。

投喂链路（Bundle vs Run、compiler、Slave 读 phase 文件）详见 **[workflow-ingest.md](./workflow-ingest.md)**。

## 执行段：编码 → App 审核 → 进度文档

```
IDE 写出 / 下发 DAG
       ↓
App：触发 start（人工触发器）
       ↓
Slave：ready 节点 → Local Agent 编码
       ↓
节点 → awaiting_review（附变更摘要 / 文件列表 / 日志）
       ↓
App：人工审核  ──reject──► 节点 rejected（可重跑或回 IDE）
       │ approve
       ▼
Slave：更新进度文档（roadmap / milestone / phase 状态）
       ↓
节点 → approved；下游依赖满足则变 ready
       ↓
默认：**不**自动 assign 下一节点（停在 ready）
       ↓
仅当节点 `policy.autoStartNext==true`，或 App 调用
`POST …/continue` / `POST …/nodes/{id}/start` 时才续跑
```

| 角色 | 在工作流中的职责 |
|------|------------------|
| **人 / IDE** | 全部设计与拆分；规则、文档、源码上下文；DAG 与失败策略 |
| **Compiler（IDE 内 AI+工具）** | phases → DAG（不在手机上做） |
| **Slave** | 加载 DAG；调度；调 SDK 编码；**仅在 App 批准后**写进度文档；上报状态 |
| **Gateway** | 工作流实例、审核请求/结果中转、进度推送 |
| **App** | **触发器 + 进度跟踪器 + 审核器**（不做架构/拆 phase） |

### App 三能力（冻结）

1. **触发**：启动工作流 / 启动下一节点 / 取消  
2. **进度**：DAG 节点状态、当前 running、阻塞在审核还是失败  
3. **审核**：
   - **只读可视 diff**（看变更，不在 App 编辑源码）  
   - **输入框**：通知 Slave「如何修改」→ Slave 再跑一轮 Agent  
   - **通过**：批准后才写进度文档并解锁下游  

审核通过前：

- 不把节点标为业务成功  
- **不更新** roadmap / milestone / phase 进度文档  
- 下游节点不解锁  

审核中的修改循环：

```
awaiting_review →（App 输入修改意见）→ running → awaiting_review → …
                      └─（App 通过）→ approved + 写进度文档
```

### 进度文档由谁写

| 动作 | 谁 |
|------|-----|
| 设计内容（目标、DoD、依赖） | IDE 人+AI |
| 运行进度（phase done、milestone 百分比、勾选） | **Slave，且仅在 App approve 之后** |

进度文档路径在 DAG/配置里声明（例如 `roadmaps/.../README.md` 或专用 `progress.md`），避免 Slave 乱改设计正文。

单次「自由 prompt」任务仍可保留；默认也建议走同一套「跑完 → App 审核」闸门（可配置关闭）。

## 与现有「单任务」模型的关系

| 模型 | 用途 |
|------|------|
| Task（现有 api-outline） | 一次 Agent run；DAG 的**一个 node** 执行时映射为一个 Task |
| Workflow / DAG | 多个 Task 的编排与依赖 |

建议：Gateway 同时有 `workflows` 与 `tasks`；node run 创建 task，事件仍走现有 WS `task.event`，并额外推 `workflow.event`（节点状态变更）。

## 边界再强调

| 只在 Cursor IDE（人+AI） | 只在 App | Slave 自动 |
|--------------------------|----------|------------|
| 架构、规则、文档、拆 milestone/phase | 触发执行 | 按节点调 Local Agent |
| 编 DAG、失败策略、DoD 设计 | **只读 diff + 修改意见输入框 + 通过** | 上报 diff；按意见再改 |
| 大改 phase / 重规划 | 看进度 | **approve 后更新进度文档** |

先前讨论的「phase 模板、显式依赖、运行态分离、DoD、失败策略」等风险点：**全部归设计段，在 IDE 消化**，不进手机交互范围。

## 落地顺序（建议）

1. `roadmaps/` + phase 模板（IDE）  
2. DAG schema（含 `awaiting_review` / 进度文档路径）  
3. Gateway：workflows + **review API** + 推送  
4. Slave：串行调度 + **approve 后写进度文档**  
5. App：触发 + 进度 + **审核页**（首版核心）  
6. IDE 侧 compiler 增强（可并行于上）

## 相关文档

- [architecture.md](./architecture.md) — 运行时三角：App / Gateway / Slave  
- [api-outline.md](./api-outline.md) — Task 级 API；Workflow API 后续补  
- [local-slave.md](./local-slave.md) — Slave 执行与 SDK  
- [flutter-app.md](./flutter-app.md) — App 展示与干预  
