# M01 — 仓库骨架与契约

## 目标

建立可开发的 monorepo 骨架，以及 Gateway / Slave / App 共用的契约（OpenAPI 或 JSON Schema），为后续编码提供稳定边界。

## 非目标

- 不实现真实业务逻辑  
- 不接入 Cursor SDK  
- 不做 Flutter UI  

## 依赖

无

## 验收

- 根目录可识别 `gateway/`、`slave/`、`app/`、`contracts/`、`doc/`  
- 契约中至少覆盖：Task 状态枚举、WS 事件信封、`awaiting_review` / review / revise  
- README 能说明如何进入各子项目  
- `progress.md` 中 M01 相关 phase 可被勾选机制引用  

## Phases

| Phase | 文档 |
|-------|------|
| M01-P01 | [仓库目录与根 README](../phases/M01-P01-monorepo-layout.md) |
| M01-P02 | [共享契约：Task / 事件信封](../phases/M01-P02-contracts-task-events.md) |
| M01-P03 | [共享契约：Workflow / Review / Diff](../phases/M01-P03-contracts-workflow-review.md) |
