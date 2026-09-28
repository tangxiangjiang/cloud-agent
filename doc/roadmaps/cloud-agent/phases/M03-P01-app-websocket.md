# M03-P01 — App WebSocket 枢纽

## 目标

Gateway 实现 `/v1/ws`：鉴权、subscribe/unsubscribe、ping/pong、按 taskId 推送 `task.event`（可先由内部测试注入事件）。

## 范围

**允许：** `gateway/**` WS 相关  

**禁止：** Slave 协议、SDK  

## 上下文

- [doc/communication.md](../../../../communication.md)
- contracts 事件信封

## 完成定义（DoD）

- [x] 无效 token 无法完成订阅（握手或首帧 auth 失败）
- [x] subscribe 后能收到带递增 `seq` 的事件
- [x] ping/pong 可用
- [x] 有最小手动或自动测试说明

## 禁止事项

- 在 query 中传递 Cursor API Key
