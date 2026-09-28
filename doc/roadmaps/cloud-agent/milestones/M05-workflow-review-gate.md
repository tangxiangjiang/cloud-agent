# M05 — 工作流 DAG + 审核闸门

## 目标

Gateway + Slave 支持 workflow 实例：按 DAG（首版串行）调度节点；节点跑完 → `awaiting_review`；提供只读 diff、revise 意见再跑、approve 后写 `progress.md` 并解锁下游。

## 非目标

- 不做 Flutter UI（可用 curl/HTTP 验收）  
- 不做真并行多节点同仓  
- 不在 approve 前改 progress  

## 依赖

M04

## 验收

- 可提交/加载一份样例 DAG，串行跑两节点  
- `GET .../diff` 返回可渲染的 unified diff  
- `revise` 触发 follow-up 后再进审核  
- `approve` 后 `progress.md` 对应 phase 被勾选，且第二节点才开始  
- reject 不写 progress  

## Phases

| Phase | 文档 |
|-------|------|
| M05-P01 | [Workflow/DAG 模型与 API](../phases/M05-P01-workflow-model-api.md) |
| M05-P02 | [Slave DAG 串行调度](../phases/M05-P02-slave-dag-scheduler.md) |
| M05-P03 | [Diff 采集与只读 API](../phases/M05-P03-diff-api.md) |
| M05-P04 | [Revise follow-up 循环](../phases/M05-P04-revise-loop.md) |
| M05-P05 | [Approve 写 progress.md](../phases/M05-P05-approve-progress.md) |
