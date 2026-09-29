# M09-P01 — Chat API 与 Task 桥接

## 目标

Gateway 提供 ChatSession；发消息创建/复用 Task 下发给 Slave；复用现有事件流。

## 范围

**允许：** `gateway/**`、task 字段扩展、api-outline / project-chat 文档

**禁止：** App 完整 UI；改 Workflow 审核语义

## 完成定义（DoD）

- [x] `POST /v1/chats`、`POST /v1/chats/{id}/messages` → `taskId`
- [x] Task 含 `chatId` / `mode` / `model`；`auto` 在 Slave 可解析
- [x] 取消走现有 cancel
- [x] SQLite 或内存会话（至少进程内可用；持久化可放到 P04）
- [x] 单测：创建会话 + 发消息生成 task

## 禁止事项

- App 传 cwd
- 在 chat API 打印 Bearer / API Key
