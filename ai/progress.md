# Progress (ai)

权威进度文件（Bundle.`progressDoc` 默认指向此处）。  
**仅在 App 审核通过后由 Slave 更新。**

| Bundle / Node | 状态 | 备注 |
|---------------|------|------|
| M01-P01 | approved | IDE 直接完成（App 审核闸门尚未上线） |
| M01-P02 | approved | Task/事件 JSON Schema |
| M01-P03 | approved | Workflow/Review/Diff JSON Schema |
| M02-P01 | approved | Gateway HTTP 骨架 + /v1/health |
| M02-P02 | approved | 配对鉴权 + Bearer middleware |
| M02-P03 | approved | Task CRUD / 取消 / Idempotency-Key |
| M02-P04 | approved | GET /v1/slaves 占位配置 |
| M03-P01 | approved | App WebSocket 枢纽 |
| M03-P02 | approved | Slave 出站登记 / 心跳 / online |
| M03-P03 | approved | 任务下发 + 事件 fan-out + mockslave |
| M04-P01 | approved | Slave TS 工程 + 配置 / cwd 白名单 |
| M04-P02 | approved | Gateway 出站客户端 + stub 事件 |
| M04-P03 | approved | Local Agent create/send/stream/wait |
| M04-P04 | approved | 取消 / 白名单 / 安全默认 |
| M05-P01 | approved | Workflow/DAG 模型与 API |
| M05-P02 | approved | Slave 串行 DAG 调度 + awaiting_review 闸门 |
| M05-P03 | approved | 只读 NodeDiff API + git 基线采集 |
| M05-P04 | approved | Revise follow-up + reviseHistory |
| M05-P05 | approved | Approve 写 progressDoc + 解锁下游 |
| M06-P01 | approved | Flutter 工程 + 配对登录 |
| M06-P02 | approved | 工作流列表 / 详情 / Start |
| M06-P03 | approved | App WS 日志 + afterSeq |
| M06-P04 | approved | 只读 NodeDiff 视图 |
| M06-P05 | approved | revise / approve / reject UI |
| M07-P01 | approved | 审计 JSONL + pair/revise 限流 |
| M07-P02 | approved | 样例 DAG + deploy 文档 |
| M07-P03 | approved | 文档对齐 + MVP checklist |
| M08-P01 | pending | Gateway 同步 API 与落库 |
| M08-P02 | pending | Slave 采集与上报 |
| M08-P03 | pending | 对账 warnings 与查询报告 |
| M08-P04 | pending | App 工程页同步 UI |
| M08-P05 | pending | 可选 AI 摘要 |
| M09-P01 | pending | Chat API 与 Task 桥接 |
| M09-P02 | pending | App 对话页（Agent + Auto） |
| M09-P03 | pending | Ask / Plan 与模型列表 |
| M09-P04 | pending | 会话历史与多会话 |

## 勾选

- [x] M01-P01 @approved 2026-09-28（IDE）
- [x] M01-P02 @approved 2026-09-28（IDE）
- [x] M01-P03 @approved 2026-09-28（IDE）
- [x] M02-P01 @approved 2026-09-28（IDE）
- [x] M02-P02 @approved 2026-09-28（IDE）
- [x] M02-P03 @approved 2026-09-28（IDE）
- [x] M02-P04 @approved 2026-09-28（IDE）
- [x] M03-P01 @approved 2026-09-28（IDE）
- [x] M03-P02 @approved 2026-09-28（IDE）
- [x] M03-P03 @approved 2026-09-28（IDE）
- [x] M04-P01 @approved 2026-09-28（IDE）
- [x] M04-P02 @approved 2026-09-28（IDE）
- [x] M04-P03 @approved 2026-09-28（IDE）
- [x] M04-P04 @approved 2026-09-28（IDE）
- [x] M05-P01 @approved 2026-09-28（IDE）
- [x] M05-P02 @approved 2026-09-28（IDE）
- [x] M05-P03 @approved 2026-09-28（IDE）
- [x] M05-P04 @approved 2026-09-28（IDE）
- [x] M05-P05 @approved 2026-09-28（IDE）
- [x] M06-P01 @approved 2026-09-28（IDE）
- [x] M06-P02 @approved 2026-09-28（IDE）
- [x] M06-P03 @approved 2026-09-28（IDE）
- [x] M06-P04 @approved 2026-09-28（IDE）
- [x] M06-P05 @approved 2026-09-28（IDE）
- [x] M07-P01 @approved 2026-09-28（IDE）
- [x] M07-P02 @approved 2026-09-28（IDE）
- [x] M07-P03 @approved 2026-09-28（IDE）
- [ ] M08-P01
- [ ] M08-P02
- [ ] M08-P03
- [ ] M08-P04
- [ ] M08-P05
- [ ] M09-P01
- [ ] M09-P02
- [ ] M09-P03
- [ ] M09-P04
