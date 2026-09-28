# ai/bundles

存放 **执行计划（DAG Bundle）**。

- 由 IDE Skill（`ai/skills/plan-from-roadmap`）或人工生成  
- Slave / Gateway 通过 `bundleRef`（如 `ai/bundles/m01.json`）加载  
- 字段见 [../SCHEMA.md](../SCHEMA.md)

当前可先为空；生成第一个计划后提交 JSON 即可。
