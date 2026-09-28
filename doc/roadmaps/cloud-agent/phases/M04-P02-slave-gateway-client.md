# M04-P02 — 接入 Gateway 出站客户端

## 目标

Slave 实现与 Gateway 的出站协议客户端：登记、心跳、收任务、发事件、收取消（对接 M03）。

## 范围

**允许：** `slave/**` 网络客户端；必要时微调 gateway 协议小 bug  

**禁止：** Agent.create（本 phase 可用 stub handler 回复假事件）  

## 上下文

- M03 Slave 协议文档/实现

## 完成定义（DoD）

- [ ] Slave 启动后 Gateway `GET /slaves` 显示 online
- [ ] 能领取 task 并回传至少 status 事件
- [ ] 断线重连后能重新登记
- [ ] 日志不打印完整 token/API Key

## 禁止事项

- 公网暴露 Slave HTTP 服务作为主路径
