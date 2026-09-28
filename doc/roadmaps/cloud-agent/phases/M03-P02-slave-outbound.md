# M03-P02 — Slave 出站协议与登记

## 目标

定义并实现 Slave → Gateway 出站连接：鉴权、登记 slaveId/repos、心跳、在线状态反映到 `GET /slaves`。

## 范围

**允许：** `gateway/**` slave 接入面；可选 `contracts/` 补充 slave 协议 schema；**不要**完整 Slave 业务（可用最小 ping 客户端测）  

**禁止：** `@cursor/sdk`  

## 上下文

- [doc/local-slave.md](../../../../local-slave.md)
- [doc/architecture.md](../../../../architecture.md) Slave 不对公网暴露

## 完成定义（DoD）

- [ ] 文档化出站 URL 与消息类型（register/heartbeat/…）
- [ ] Gateway 接受出站连接并更新 online
- [ ] 断开后 online=false（允许短暂宽限）
- [ ] 最小测试客户端或脚本能完成登记

## 禁止事项

- 要求 Slave 监听公网入站端口作为主路径
