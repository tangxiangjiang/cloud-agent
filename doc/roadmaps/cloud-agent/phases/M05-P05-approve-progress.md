# M05-P05 — Approve 写 progress.md

## 目标

`POST .../review` decision=approve 时：Slave **仅此时**更新 `doc/roadmaps/cloud-agent/progress.md` 对应 phase 勾选，节点 approved，下游变 ready 并可继续调度。

## 范围

**允许：** `gateway/**`、`slave/**`；只改 progress 约定格式，不改 phase 设计正文  

**禁止：** 无批准写进度；改 milestone 目标文案  

## 上下文

- [progress.md](../progress.md)
- [doc/workflow.md](../../../../workflow.md)

## 完成定义（DoD）

- [ ] approve → progress 中对应 `- [x] Mxx-Pxx` 并带时间戳或批注格式
- [ ] reject → 不改 progress
- [ ] approve 后下游节点可被调度（若仍在执行工作流）
- [ ] progress 路径由 workflow/配置声明，防止写到任意文件

## 禁止事项

- 用 AI「自由发挥」改其他 doc 设计文档冒充进度更新
