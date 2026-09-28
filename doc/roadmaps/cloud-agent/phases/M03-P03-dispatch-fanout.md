# M03-P03 — 任务下发 + 事件转发联调（mock slave）

## 目标

queued Task 下发给已登记 Slave；Slave（mock）上报事件；Gateway 写入事件流并 fan-out 到 App WS 订阅者。

## 范围

**允许：** `gateway/**`；`slave/` 下可用 **mock** 客户端（不调 SDK）  

**禁止：** 真实 Local Agent  

## 上下文

- M03-P01、M03-P02
- Task 状态机

## 完成定义（DoD）

- [x] 创建 task → mock slave 收到 → 回报 running/delta/done
- [x] App WS 订阅者能看到对应事件
- [x] cancel 能下达到 mock slave（mock 标记 cancelled 即可）
- [x] `GET /tasks/{id}/events?afterSeq=` 可用作兜底（若已实现）

## 禁止事项

- 跳过 Gateway 让「App 直连 mock」
