# M05-P02 — Slave DAG 串行调度

## 目标

Slave 加载 workflow：按拓扑取 ready 节点，**串行**执行（一节点一轮 Agent/task），状态同步 Gateway。

## 范围

**允许：** `slave/**`、gateway 下发/状态同步所需小改  

**禁止：** 同仓真并行；跳过 review 直接 approved  

## 上下文

- M04 Local Agent
- M05-P01

## 完成定义（DoD）

- [x] 两节点链式 DAG：第一节点跑完进入 awaiting_review 后，第二节点仍不得 running
- [x] 每节点映射到 taskId（可查）
- [x] 失败节点按策略 stop（首版默认 stop 即可）
- [x] 有一份 fixtures 样例 DAG JSON

## 禁止事项

- Agent 结束后直接标 approved
