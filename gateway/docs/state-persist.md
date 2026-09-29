# Gateway / Slave 本地状态持久化

重启后审核进度不应回退。状态分两处落盘（均在仓库 `.local/`，gitignored）：

| 组件 | 文件 | 内容 |
|------|------|------|
| Gateway | `.local/gateway/state.json` | workflows、node diffs、pair tokens |
| Slave | `.local/slave-runtime.json` | revise baselines、agentId（便于 resume） |

## 行为

- `python run.py up` 自动传 `-state-file` / `SLAVE_STATE_FILE`
- Gateway 每次 create/patch/review/diff/pair 后 debounce 写盘
- 启动时恢复；`running` 且已有 diff 的节点会收成 `awaiting_review`，否则 `ready`
- **progress.md / git commits** 仍在目标仓库磁盘上，与 Gateway 快照独立

## 手动启动 Gateway

```bash
go run . -pair-code ABCD-EFGH -state-file ../.local/gateway/state.json
```

无 `-state-file` 时仍为纯内存（重启丢进度）。
