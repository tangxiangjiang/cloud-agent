# M09 — 工程 AI 对话

## 目标

App 在 Slave→工程进入类 Cursor 对话页：支持 **Agent / Ask / Plan**、**模型选择（默认 Auto）**，经 Gateway 驱动本机 Local Agent，流式回传。

设计权威文档：[../../project-chat.md](../../project-chat.md)。

## 非目标

- 不做 Cloud Agent / 手机持 Key  
- 不做完整 IDE（@文件树、Inline Edit）  
- 首版不强制对话改码走 awaiting_review（可选后续）  

## 依赖

M06（App WS / Task 流）；建议与 M08 并行或之后（工程页入口已存在）。

## 验收

- 工程页可进对话，发送后看到流式回复  
- Agent / Ask / Plan 切换影响后续发送（至少 prompt 可区分）  
- 模型默认 Auto；可选具体 model  
- 可停止生成；Key 不进 App  

## Phases

| Phase | 文档 |
|-------|------|
| M09-P01 | [Chat API 与 Task 桥接](../phases/M09-P01-chat-api.md) |
| M09-P02 | [App 对话页（Agent + Auto）](../phases/M09-P02-app-chat-ui.md) |
| M09-P03 | [Ask / Plan 与模型列表](../phases/M09-P03-modes-models.md) |
| M09-P04 | [会话历史与多会话](../phases/M09-P04-chat-history.md) |
