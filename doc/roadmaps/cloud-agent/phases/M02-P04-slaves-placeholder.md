# M02-P04 — Slaves 列表占位 API

## 目标

实现 `GET /v1/slaves`，返回可配置的占位 slave/仓库列表（为 App 与后续真实登记打底）。

## 范围

**允许：** `gateway/**`、配置文件如 `gateway/config.example.yaml`  

**禁止：** Slave 进程实现  

## 上下文

- [doc/api-outline.md](../../../../api-outline.md) Slaves 小节

## 完成定义（DoD）

- [ ] 鉴权后可 GET slaves
- [ ] JSON 含 id/name/online/repos[{id,name,cwd}]
- [ ] online 首版可为配置静态值或 false（M03 再接真实在线）
- [ ] 与 contracts 或 api-outline 字段名一致（`slaves` 而非 workers）

## 禁止事项

- 把本机真实任意路径未白名单化地写死进默认生产配置而不说明
