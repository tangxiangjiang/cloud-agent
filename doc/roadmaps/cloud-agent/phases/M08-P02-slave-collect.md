# M08-P02 — Slave 采集与上报

## 目标

Slave 收到 `project.sync` 后，在白名单 cwd 内做**结构化只读采集**（git / milestones / progress），上报 Gateway；无 API Key 时规则摘要即可。

## 范围

**允许：** `slave/**`、必要的 gateway 协议字段对齐

**禁止：** App UI；强制依赖 Local Agent（本 phase 可不跑 AI）

## 上下文

- [doc/project-sync.md](../../../project-sync.md)「采集内容」
- `projects[].index` / `progressDoc`

## 完成定义（DoD）

- [x] 处理 WS `project.sync`；`repoId` 必须命中白名单
- [x] 采集：`branch` / `head` / `dirty` / milestones 索引 / progress phases / 可选 `git log -n 10`
- [x] 组装 `ProjectSyncPayload`（schemaVersion=1）并 `POST /v1/project-sync`（或 WS result）
- [x] 失败时上报 error 字段，不崩溃进程
- [x] 单测：无 git / 无 progress 文件时的降级
- [x] 默认不写工作区文件

## 禁止事项

- 采集阶段 `git commit` / 改 progress.md
- 把完整 unified diff 或密钥打进 payload
