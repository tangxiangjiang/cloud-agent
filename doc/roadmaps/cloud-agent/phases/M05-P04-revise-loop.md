# M05-P04 — Revise follow-up 循环

## 目标

实现 `POST .../revise`：将 `instruction` 交给 Slave，对当前节点 agent follow-up / 再跑，完成后刷新 diff 并回到 awaiting_review。

## 范围

**允许：** `gateway/**`、`slave/**`  

**禁止：** 写 progress.md；Flutter  

## 上下文

- workflow 审核循环
- SDK `agent.send` follow-up / resume

## 完成定义（DoD）

- [ ] revise 仅在 awaiting_review 允许
- [ ] 节点状态 running → 再 awaiting_review
- [ ] diff 内容更新
- [ ] instruction 进入审计或事件（勿丢）
- [ ] 测试或脚本演示一轮 revise

## 禁止事项

- revise 直接 approve
