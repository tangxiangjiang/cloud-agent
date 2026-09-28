# ai/ — 工程内 AI 工作区索引

本目录是 **AI 计划与任务单元** 的根：IDE Skill 写入、Slave 读取/生成、再经 Gateway 推到手机。  
设计长文仍在 `doc/`；**可执行、可下发** 的东西优先落在这里。

## 目录

| 路径 | 用途 |
|------|------|
| [README.md](./README.md) | 本索引 |
| [SCHEMA.md](./SCHEMA.md) | Bundle / TaskUnit 字段约定 |
| [bundles/](./bundles/) | 执行计划（DAG），Skill 或人生成 |
| [units/](./units/) | **任务单元**：Slave 结构化产出，供手机展示与审核 |
| [skills/](./skills/) | 本仓库 Skill 源码（如 plan-from-roadmap） |
| [progress.md](./progress.md) | AI 执行进度勾选（App 批准后由 Slave 更新） |

设计 roadmap（milestone / phase 说明文）见：[doc/roadmaps/cloud-agent/](../doc/roadmaps/cloud-agent/README.md)

## 数据流（冻结）

```
doc/roadmaps + doc/*     ← 人在 IDE 做架构 / milestone / phase 文
        │
        ▼
ai/skills/* + Cursor     ← Skill：生成/更新 ai/bundles/*.json
        │
        ▼
ai/bundles/<plan>.json   ← 计划（DAG）
        │  start
        ▼
Slave（本机，可读全仓）
        │  按节点执行 Local Agent
        │  写出结构化任务单元
        ▼
ai/units/<workflowId>/<nodeId>.json
        │  经 Gateway 推送
        ▼
手机 App：进度 / 只读 diff / 修改意见 / 通过
        │  approve
        ▼
Slave 更新 ai/progress.md（及约定的 roadmap progress）
```

## Slave / 手机认什么

- **开始跑**：`ai/bundles/*.json`（或 Gateway Run 里对该文件的快照）  
- **推到手机的卡片**：`ai/units/**/*.json`（见 SCHEMA）+ diff 元数据  
- **不要**让 App 上传整仓；本机已有全工程  

## 快速入口

1. 写计划：在 Cursor 使用 [skills/plan-from-roadmap](./skills/plan-from-roadmap/SKILL.md)  
2. 看计划：[`bundles/`](./bundles/)  
3. 看下发单元：[`units/`](./units/)  
4. 看字段：[`SCHEMA.md`](./SCHEMA.md)  
