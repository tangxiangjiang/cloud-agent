# Gateway / Slave 本地状态持久化

重启后审核进度不应回退。Gateway 优先用 **SQLite**；Slave 仍用 JSON 运行时文件。

| 组件 | 文件 | 内容 |
|------|------|------|
| Gateway | `.local/gateway/state.db` | SQLite：workflows / diffs / tokens；`project_sync`（工程状态同步，M08-P01） |
| Gateway（旧） | `.local/gateway/state.json` | 首次打开 `state.db` 且库为空时自动导入，并改名为 `state.json.migrated` |
| Slave | `.local/slave-runtime.json` | revise baselines、agentId（便于 resume） |

## 行为

- `python run.py up` 自动传 `-state-file …/state.db`
- Gateway 每次 create/patch/review/diff/pair 后 debounce 写库（事务替换）
- 启动时恢复；`running` 且已有 diff 的节点会收成 `awaiting_review`，否则 `ready`
- **progress.md / git commits** 仍在目标仓库磁盘上，与 Gateway 快照独立
- 仍可用 `-state-file …/state.json` 走旧 FileStore

## 手动启动 Gateway

```bash
go run . -pair-code ABCD-EFGH -state-file ../.local/gateway/state.db
```

无 `-state-file` 时仍为纯内存（重启丢进度）。

## 后续：工程「同步」按钮

完整设计见 **[doc/project-sync.md](../../doc/project-sync.md)**。

摘要：App 在 Slave→工程上触发同步 → Slave 采集 milestone/progress/git（可选 AI 摘要）→ Gateway 写入 `project_sync`，并与 active Workflow 对账告警。**Gateway 触发/落库/查询 API 已实现（M08-P01）**；Slave 采集与 App UI / 对账见后续 phase。
