# ai/bundles

存放 **执行计划（DAG Bundle）**。

- 由 IDE Skill（`ai/skills/plan-from-roadmap`）或人工生成  
- Slave / Gateway 通过 `bundleRef`（如 `ai/bundles/m01.json`）加载  
- 字段见 [../SCHEMA.md](../SCHEMA.md)

样例：`m05-p02-two-node.json`（两节点串行；N1→`awaiting_review` 后 N2 不得跑，直至 approve）。
