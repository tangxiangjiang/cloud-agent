# M06 — Flutter App MVP

## 目标

Flutter App：配对、工作流进度、触发、节点只读 diff、修改意见输入框、通过；经 Gateway HTTP+WS，不持有 API Key。

## 非目标

- 不做 IDE 级编辑  
- 不做架构/phase 编辑  
- 不做复杂 DAG 可视化编辑器（列表+状态即可）  

## 依赖

M05

## 验收

- 真机或模拟器：配对 → 看工作流 → 触发 → 看日志 → 看 diff → revise → 再审 → approve → 进度前进  
- Token 进安全存储  
- 正式路径仅 HTTPS/WSS 可配置  

## Phases

| Phase | 文档 |
|-------|------|
| M06-P01 | [Flutter 工程与配对登录](../phases/M06-P01-app-pair.md) |
| M06-P02 | [工作流列表/详情与触发](../phases/M06-P02-workflow-ui.md) |
| M06-P03 | [WS 日志订阅](../phases/M06-P03-ws-logs.md) |
| M06-P04 | [只读 diff 视图](../phases/M06-P04-readonly-diff.md) |
| M06-P05 | [意见框 revise + 通过](../phases/M06-P05-revise-approve.md) |
