# 工作流投喂：IDE Skill → ai/ → Slave → 手机

Slave 与 IDE 同机、可扫全工程。生成计划用 **Cursor Skill**；可执行产物统一落在仓库根目录 **`ai/`**。

## 两件事

| 事 | 怎么做 |
|----|--------|
| **生成计划** | Skill → `ai/bundles/*.json`（人确认） |
| **执行与手机审核** | Slave 跑节点 → 写 `ai/units/**` → Gateway → App |

索引与字段：**[ai/README.md](../ai/README.md)**、**[ai/SCHEMA.md](../ai/SCHEMA.md)**

## 流

```
doc/roadmaps（设计）
    → Skill plan-from-roadmap
    → ai/bundles/*.json
    → Gateway start(bundleRef)
    → Slave 执行
    → ai/units/<wf>/<node>.json 推送手机
    → App：只读 diff / 意见 / 通过
    → Slave 更新 ai/progress.md
```

`bundleRef` 示例：`ai/bundles/m01.json`  
不必 App 传源码；不必独立 compiler 服务。

## Skill

- 源码：`ai/skills/plan-from-roadmap/SKILL.md`  
- 已安装到：`.cursor/skills/plan-from-roadmap/`（便于 Agent 发现）  

## 相关

- [workflow.md](./workflow.md) — 审核状态机  
- [local-slave.md](./local-slave.md) — 执行  
- [flutter-app.md](./flutter-app.md) — 手机三能力  
