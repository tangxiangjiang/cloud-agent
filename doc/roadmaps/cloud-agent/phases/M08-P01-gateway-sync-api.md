# M08-P01 — Gateway 同步 API 与落库

## 目标

实现工程同步的 Gateway 侧入口与 SQLite 写入：接收 App 触发、向 online Slave 下发 `project.sync`，接收 Slave 上报并 UPSERT `project_sync`。

## 范围

**允许：** `gateway/**`、`doc/api-outline.md`、`gateway/docs/**`

**禁止：** App UI、Slave AI 摘要逻辑（可先留 WS/HTTP 桩）

## 上下文

- [doc/project-sync.md](../../../project-sync.md)
- [gateway/docs/state-persist.md](../../../../gateway/docs/state-persist.md)（`project_sync` 表）

## 完成定义（DoD）

- [ ] `POST /v1/slaves/{slaveId}/projects/{repoId}/sync`（Bearer）；Slave offline → 明确错误
- [ ] Gateway → Slave WS：`project.sync`（`requestId`, `repoId`）
- [ ] `POST /v1/project-sync`（或等价 WS `project.sync.result`）校验 `slaveId`/`repoId`，写入 SQLite
- [ ] `GET /v1/slaves/{slaveId}/projects/{repoId}/sync` 返回最近快照
- [ ] 审计事件：`project.sync` accept / result（无 token / API Key）
- [ ] 单测覆盖落库与 offline

## 禁止事项

- 用 App 传入的任意 cwd 覆盖白名单
- 在同步 API 中改写 Workflow 节点 status
