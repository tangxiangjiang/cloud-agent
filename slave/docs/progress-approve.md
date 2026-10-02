# Approve 写 progress + 本地 git commit（M05-P05 扩展）

## 流程

```
awaiting_review
  → App POST /v1/workflows/{id}/nodes/{nodeId}/review { "decision": "approve" }
  → Gateway: 节点 → approved；RecomputeReady（下游 pending→ready）
  → WS workflow.review (approve) + workflow.assign
  → Slave:
      1) 更新 Bundle.progressDoc（basename 必须为 progress.md）
      1b) 若节点 `phaseRef` 文内有 `Status:` 字段：待审时改为 Ready for human review；approve 时改为 Done/done（消双 SoT 剪刀差；不改设计正文）
      2) 本地 git：AI 写 commit message → git add -A → git commit（不 push；含 phase Status 变更）
  → Slave: 继续调度新 ready 节点
```

进入 `awaiting_review` 时也会尽量回写 phaseRef 的 `Status:`（若存在），便于人读文档与 Gateway 状态对齐。
`decision: reject` → 节点 `rejected`，**不**写 progress，**不** commit，下游不解锁。

## 反盖章（空转拦截）

| 层 | 规则 |
|----|------|
| Slave 待审前 | `assessNodeDiff`：空 diff / 仅 `progress.md` → 节点 `failed`，**不**进 `awaiting_review`；evidence/harness phase 还要求至少有代码/harness 产物变更 |
| Gateway approve | 空 diff / 仅 `progress.md` → HTTP 409，拒绝 approve |
| Gateway autoApprove | 同上条件不满足则 **跳过** autoApprove，节点停在 `awaiting_review` |
| Slave 收 approve | 再验一次 stored diff；非实质则 **不写** progress.md、不 commit |

## 本地 commit

| 项 | 说明 |
|----|------|
| 时机 | 仅 approve，且在写完 progress.md 之后 |
| 范围 | 白名单 `repos[].cwd` 工作区全部变更（含 progress 勾选） |
| 消息 | 有 `CURSOR_API_KEY` 时用 Local Agent 根据 `git status` / `diff --stat` 生成；失败或 `--stub` 时用 `approve(Mxx-Pxx): <title>` |
| 推送 | **不** `git push` |
| 干净树 | 无变更则跳过 commit（打日志） |

## 路径约束

| 规则 | 说明 |
|------|------|
| 来源 | 仅 `WorkflowRun.progressDoc`（创建工作流时声明） |
| 相对路径 | 相对仓库白名单 `cwd`，禁止绝对路径与 `..` 逃逸 |
| 文件名 | basename 必须为 `progress.md`（防止改 phase 设计正文） |

## 勾选格式

从节点 `id` / `unitId` / `phaseRef` 文件名解析 `Mxx-Pxx`，写入：

```markdown
- [x] M05-P05 @approved 2026-09-28T12:00:00.000Z
```

若存在 `| M05-P05 | … |` 表格行，将状态格改为 `approved`。不改 milestone 目标文案、不改其他设计文档。
