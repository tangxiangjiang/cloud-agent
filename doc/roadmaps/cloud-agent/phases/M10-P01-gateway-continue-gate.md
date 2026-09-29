# M10-P01 — Gateway 续跑门闩

## 目标

Approve 后仅当节点 `policy.autoStartNext==true`（或显式 continue/start API）才 `AssignWorkflow`；默认停在下游 `ready`。

## 范围

**允许：** `gateway/internal/workflow/**`、api-outline、node-policy 文档

## 完成定义（DoD）

- [x] 节点 schema / 创建请求支持 `policy.autoStartNext`（默认 false）
- [x] Approve 路径：无 autoStartNext 时不自动 assign
- [x] `POST .../continue` 或 `.../nodes/{id}/start` 可续跑
- [x] 单测：approve 后不 assign；flag true 时 assign
- [x] 更新 [workflow.md](../../../workflow.md) / 部署说明中的行为描述

## 禁止事项

- 默认 true 以「兼容旧习惯」偷偷保留自动续跑（须显式开）
