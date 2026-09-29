# M09-P04 — 会话历史与多会话

## 目标

ChatSession 与消息摘要持久化（Gateway SQLite）；App 可列出同工程历史会话并继续。

## 范围

**允许：** `gateway/internal/persist`、chat store、`app` 历史抽屉

## 完成定义（DoD）

- [ ] 重启 Gateway 后会话列表可恢复
- [ ] `GET /v1/chats?repoId=`；打开历史可续聊（follow-up / 新 task）
- [ ] 消息存储截断策略文档化（防爆库）

## 禁止事项

- 持久化完整 tool 原始 payload / 密钥
