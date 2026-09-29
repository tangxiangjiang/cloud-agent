# M09-P02 — App 对话页（Agent + Auto）

## 目标

工程页进入对话 UI：消息列表、输入发送、订阅 task 流式输出；模式先固定 Agent，模型默认 Auto。

## 范围

**允许：** `app/**`

**禁止：** 持有 CURSOR_API_KEY；Ask/Plan 完整策略（下一 phase）

## 完成定义（DoD）

- [ ] Slave→工程→「对话」入口
- [ ] 发送 → 展示 user + streaming assistant
- [ ] running 时可停止
- [ ] 新对话清空本地视图并 `POST /chats`
- [ ] 基础 widget/API 测试

## 禁止事项

- 对话失败时误建 Milestone Workflow
