# M10 — 节点执行策略（模型 / 自动通过 / 自动续跑）

## 目标

工作流每个任务行可配置：**模型（默认 Auto）**、**自动通过**、**自动开始下个任务**（两 Switch 默认关）。Approve 后不再无条件自动开跑下游，除非打开「自动下个」或用户手动继续。

设计权威文档：[../../node-policy.md](../../node-policy.md)。

## 非目标

- 不取消人工 Reject  
- 不做并行多 ready 同时跑（仍串行调度）  
- 自动通过不得跳过 progress/commit 管道  

## 依赖

M05（审核闸门）、M06（工作流详情 UI）

## 验收

- 默认：Approve 后下一节点为 `ready`，不自动开始  
- 打开「自动下个」后行为接近现网自动续跑  
- 节点可选模型 Auto / 具体 id  
- 「自动通过」关时仍须人手 Approve；开时有审计  

## Phases

| Phase | 文档 |
|-------|------|
| M10-P01 | [Gateway 续跑门闩](../phases/M10-P01-gateway-continue-gate.md) |
| M10-P02 | [App 节点策略控件](../phases/M10-P02-app-node-policy-ui.md) |
| M10-P03 | [自动通过](../phases/M10-P03-auto-approve.md) |
| M10-P04 | [创建/批量默认策略](../phases/M10-P04-default-policy.md) |
