# Flutter 终端

## 定位（冻结）

手机端是三件事，不多做：

1. **人工触发器** — 启动工作流 / 节点、取消  
2. **进度跟踪器** — 看 DAG / milestone 走到哪  
3. **审核器** — AI 写完代码后，人确认通过或驳回  

不做：架构设计、拆 phase、编 DAG、改规则文档。那些在 Cursor IDE（人+AI，且工程内规则/文档/源码已齐全）。

不持有 `CURSOR_API_KEY`；不直连 Slave。

## 建议页面结构

| 页面 | 作用 |
|------|------|
| 配对 / 登录 | 网关地址 + 配对码 → token |
| Slave 列表 | 在线 Slave、仓库别名 |
| 工作流列表 | 可执行 DAG 实例 |
| 工作流详情 | 节点状态条/列表；触发开始；看卡在哪 |
| **节点审核** | 变更摘要、文件列表、测试/日志要点；**通过 / 驳回** |
| 任务/节点日志 | 流式输出（审核前也可看） |
| 设置 | 网关 URL、通知、凭证 |

首版主路径：配对 → 工作流详情 →（跑完）**审核** → 通过后进度前进。  
实现见 `app/`（M06）：配对、列表/详情/Start、WS 日志、只读 diff、revise/approve。联调：[deploy.md](./deploy.md)。

工作流与审核闸门见 [workflow.md](./workflow.md)。

## 审核页（核心能力）

Agent 节点进入 `awaiting_review` 后进入审核。能力冻结为：

1. **只读可视 diff**（可看、不可编辑）  
2. **输入框**：把「如何改」发给 Slave，由 Local Agent 按意见再改一轮  
3. **通过**：批准本节点，Slave 才更新进度文档  

| 展示 | 说明 |
|------|------|
| Phase / 节点标题与目标摘要 | 来自 DAG / phaseRef |
| 变更文件列表 | 可点开 |
| **可视 diff（只读）** | unified / split 任一；支持文件间切换；**无编辑、无内联改码** |
| 自动检查结果 | 若有 `dodChecks` |
| 助手结束摘要 | 可选 |

| 操作 | 效果 |
|------|------|
| **通过** | Gateway → Slave：更新进度文档 → `approved` → 解锁下游 |
| **按意见修改**（输入框提交） | Gateway → Slave：把文本作为 follow-up / 新一轮 `send`；节点回到 `running`，改完再进 `awaiting_review` |
| **驳回 / 中止**（可选） | `rejected`，不写进度文档；可之后再触发 |

输入框是远程改码指令通道，不是文档编辑器；**不在 App 上改源文件内容**。

## 与通信层的配合

```
UI ──► WorkflowRepository / TaskRepository
          ├─ HttpApi.startWorkflow / getWorkflow
          ├─ HttpApi.reviewNode(approve|reject)
          └─ WsClient.subscribe(workflowId|taskId) → events
```

- 节点 `awaiting_review` 时推送通知（若系统允许）  
- 回前台：HTTP 拉工作流快照，再订阅 WS  

详见 [communication.md](./communication.md)。

## 状态展示建议

| 节点状态 | UI |
|----------|-----|
| `pending` / `ready` | 等待 |
| `running` | 进行中 + 可进日志 |
| `awaiting_review` | **待审核**（主 CTA） |
| `approved` | 已通过 |
| `rejected` | 已驳回（重跑 / 回 IDE） |
| `failed` / `cancelled` | 失败 / 取消 |

## 非功能

- Token：`flutter_secure_storage`  
- 仅 HTTPS/WSS（正式包）  
- 后台不保证长连接；回前台 HTTP 对齐  

## 明确不做（手机）

- 编辑 phase / DAG / 架构文档  
- **在 diff 里直接改代码**（只读可视化）  
- 直连 Slave  
