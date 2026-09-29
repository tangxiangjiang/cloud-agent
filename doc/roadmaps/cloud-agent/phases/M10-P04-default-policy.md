# M10-P04 — 创建/批量默认策略

## 目标

创建 Milestone Workflow 或「执行 plan」页可设默认模型与两开关；写入各节点 `policy` / `model`。

## 范围

**允许：** `app/**`（milestone_phases / 创建请求）、gateway 创建默认继承

## 完成定义（DoD）

- [ ] 创建请求支持 `defaultPolicy` / `defaultModel` 或节点级显式字段
- [ ] UI 可批量设「本 Milestone 默认」
- [ ] 文档示例更新

## 禁止事项

- 批量默认强制为自动通过且无法关闭
