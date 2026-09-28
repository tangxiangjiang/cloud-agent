# M06-P03 — WS 日志订阅

## 目标

App 订阅工作流当前 task/节点日志：assistant.delta 等实时展示；回前台 HTTP 对齐。

## 范围

**允许：** `app/**`  

**禁止：** 修改 Gateway 协议大改（仅适配已有 WS）  

## 上下文

- [doc/communication.md](../../../../communication.md)
- M03 App WS

## 完成定义（DoD）

- [x] 节点 running 时可看流式日志
- [x] 断线重连或 afterSeq/快照策略可用
- [x] 离开页面取消订阅，避免泄漏

## 禁止事项

- 用轮询替代 WS 作为唯一正式方案（轮询仅兜底）
