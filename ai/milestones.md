# 工程 Milestone 索引

路径由 Slave `projects[].index` 指定（相对工程 `cwd`），默认建议：`ai/milestones.json`。

与 [`index.json`](./index.json)（`run.py --plan` 的 Bundle 目录）分开：本文件只描述**本工程**的 milestone → phases，供 App「Slave → Milestone → 启动」使用。

## 字段

| 字段 | 说明 |
|------|------|
| `milestones[].id` | 如 `M07` |
| `milestones[].title` | 展示名 |
| `milestones[].progressDoc` | 可选；默认 `ai/progress.md` |
| `milestones[].phases[]` | 串成 Workflow 节点：`id` / `title` / `phaseRef` / `dependsOn` / `model` / `prompt` |

App 点击 Milestone 后：`POST /v1/workflows`（nodes=phases）并 Start，之后走现有 Diff / Review / Approve。
