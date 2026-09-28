# M03 — Gateway WS + Slave 协议

## 目标

Gateway 提供 App 侧 WebSocket 订阅与事件推送；定义并实现 Slave **出站**连接协议（登记、拉任务、上报事件、接收取消）。

## 非目标

- Slave 内不跑 Cursor SDK（可用 mock 上报假事件）  
- 不做审核/diff API  
- 不做 Flutter  

## 依赖

M02

## 验收

- App（或 wscat）可连 `wss?/v1/ws`，subscribe task 后收到 `task.event`  
- Mock Slave 出站连上后能领取 queued task 并回传事件，Gateway 转发给订阅者  
- 断线重连后 `lastSeq` 或 HTTP events 快照可对齐（至少一种可用）  

## Phases

| Phase | 文档 |
|-------|------|
| M03-P01 | [App WebSocket 枢纽](../phases/M03-P01-app-websocket.md) |
| M03-P02 | [Slave 出站协议与登记](../phases/M03-P02-slave-outbound.md) |
| M03-P03 | [任务下发 + 事件转发联调（mock slave）](../phases/M03-P03-dispatch-fanout.md) |
