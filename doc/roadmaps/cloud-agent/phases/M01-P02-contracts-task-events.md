# M01-P02 — 共享契约：Task / 事件信封

## 目标

在 `contracts/` 定义 Task 状态与 WebSocket/HTTP 事件信封的 JSON Schema（或等价 OpenAPI 片段），与 api-outline 对齐。

## 范围

**允许：** `contracts/**`、必要时更新 `doc/api-outline.md` 小处对齐  

**禁止：** Gateway/Slave/App 业务实现  

## 上下文

- [doc/api-outline.md](../../../../api-outline.md)
- [doc/communication.md](../../../../communication.md)

## 完成定义（DoD）

- [x] 存在 Task 状态枚举（含 queued/running/cancelling/finished/error/cancelled 等）
- [x] 存在事件信封字段：`type`、`taskId`、`seq`、`at`、`event.kind`、`payload`
- [x] `event.kind` 含 status / assistant.delta / tool.* / error / done
- [x] README 或 contracts/README 说明如何校验 schema

## 禁止事项

- 不要只写 Markdown 而无机器可读 schema
