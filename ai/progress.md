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
- [ ] （其余 phase：Slave 在 App approve 后写入，格式：`- [x] <nodeId> @approved <iso8601>`）
