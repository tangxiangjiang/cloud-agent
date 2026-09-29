# AI 数据结构约定

机器可读约定；实现以 `contracts/`（M01）为准时，以 contracts 为最终 schema，本文件为工程内说明。

## Bundle（执行计划，DAG）

路径：`ai/bundles/<id>.json`

生成后须登记到 **`ai/index.json`**（人读：`ai/INDEX.md`），用短名 `plans[].id` 启动：

`python run.py up --plan <id>`

```json
{
  "schemaVersion": 1,
  "id": "bundle_m01",
  "title": "M01 仓库骨架与契约",
  "repoId": "r_cloud_agent",
  "preferredSlaveId": "slave_devpc",
  "progressDoc": "ai/progress.md",
  "roadmapRef": "doc/roadmaps/cloud-agent/README.md",
  "nodes": [
    {
      "id": "M01-P01",
      "phaseRef": "doc/roadmaps/cloud-agent/phases/M01-P01-monorepo-layout.md",
      "title": "仓库目录与根 README",
      "dependsOn": [],
      "model": "composer-2.5",
      "dodChecks": [],
      "onFailure": "stop",
      "prompt": {
        "mode": "phase_file",
        "extra": "遵守 phase 范围与禁止事项。"
      }
    }
  ]
}
```

## TaskUnit（任务单元 → 手机）

路径：`ai/units/<workflowId>/<nodeId>.json`  
由 **Slave** 在节点进入 `awaiting_review`（或 running 更新）时写出，并经 Gateway 推送 App。

```json
{
  "schemaVersion": 1,
  "unitId": "unit_wf1_M01-P01",
  "workflowId": "wf_xxx",
  "nodeId": "M01-P01",
  "bundleId": "bundle_m01",
  "status": "awaiting_review",
  "title": "仓库目录与根 README",
  "phaseRef": "doc/roadmaps/cloud-agent/phases/M01-P01-monorepo-layout.md",
  "summary": "已创建 monorepo 目录与根 README。",
  "changedFiles": [
    { "path": "README.md", "additions": 20, "deletions": 0 }
  ],
  "diffRef": "gateway://workflows/wf_xxx/nodes/M01-P01/diff",
  "taskId": "tsk_xxx",
  "createdAt": "2026-09-28T00:00:00.000Z",
  "updatedAt": "2026-09-28T00:05:00.000Z"
}
```

| 字段 | 说明 |
|------|------|
| `status` | 与节点状态一致：`running` / `awaiting_review` / `approved` / … |
| `summary` | 给手机列表/卡片的短摘要 |
| `changedFiles` | 只读预览用；完整 patch 走 diff API |
| `diffRef` | App 拉只读 diff 的逻辑引用 |

App **不编辑**源码；修改意见走 `revise`，通过走 `approve`。

## 进度

`ai/progress.md`：Approve 后由 Slave 勾选对应 `nodeId` / phase id。  
可与 `doc/roadmaps/cloud-agent/progress.md` 同步或二选一；Bundle 的 `progressDoc` 指向权威文件（默认 `ai/progress.md`）。
