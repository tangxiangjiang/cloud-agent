# M10-P03 — 自动通过

## 目标

`policy.autoApprove==true` 时，节点进入 `awaiting_review` 后走与人工 Approve 相同管道（progress + commit + 审计 `autoApprove`）。

## 范围

**允许：** `gateway/**`、必要时 Slave 配合；审计字段

## 完成定义（DoD）

- [ ] autoApprove 触发与人工 approve 副作用一致
- [ ] 审计可区分自动 / 人工
- [ ] 与 autoStartNext 组合单测（开/关矩阵至少 2 格）
- [ ] Reject 不受 autoApprove 影响

## 禁止事项

- 跳过 progress.md / git commit
- 自动通过时不写审计
