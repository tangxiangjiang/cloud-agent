# M11-P04 — Gateway Master 通道与 HTTP

## 目标

Gateway 接纳 Master 出站连接，镜像配置与运行态；提供 App 用的 `/v1/masters*`，并把 CRUD / 启停转发到 Master。

## 范围

**允许：** `gateway/**`、`gateway/docs/**`、`doc/api-outline.md`、契约 schema  

**禁止：** App UI；改 Task/Workflow 数据面路由语义（仍按子 `slaveId`）

## 上下文

- [doc/slave-master.md](../../../slave-master.md) 控制面协议
- 现有 Slave outbound hub（可并列 Master hub）

## 完成定义（DoD）

- [ ] Master WS：auth（**与 Slave 同一配对 token**）→ register → heartbeat；`slaves.report`
- [ ] 控制消息带 `requestId`；HTTP **同步等待** `*.ok`/`*.error`，默认 **30s → 504**
- [ ] `stop` 编排：对该 `slaveId` **尽力 cancel 进行中 tasks**，再等 Master 完成 grace/kill
- [ ] 镜像配置 + `process`；GET **合并** `gatewayOnline`
- [ ] HTTP：`/v1/masters*` CRUD + start/stop/restart；限流；错误码见设计 §14
- [ ] Master offline → `409`；未送达 → `503`
- [ ] `models.refresh`：任一 online 子 Slave（取最新 report）
- [ ] 审计：`master.register` / `slave.config.*` / `slave.control.*`（无密钥）
- [ ] 单测：register → GET；同步 control；三态；超时 504

## 禁止事项

- App 请求体携带 `CURSOR_API_KEY`
- 用 Master 连接冒充子 Slave 领取 `task.assign`
- 把 Gateway 镜像当成可写权威
- 控制面默认 202+让 App 自建轮询（M11）
