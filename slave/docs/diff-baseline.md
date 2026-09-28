# Diff 基线策略（M05-P03）

## 目标

节点进入 `awaiting_review` 时，App 通过只读 `GET /v1/workflows/{id}/nodes/{nodeId}/diff` 查看变更。  
**没有** apply-patch / 写回源码的 HTTP API。

## 基线

| 时机 | 动作 |
|------|------|
| 节点开始执行前（`running`） | Slave 在仓库 `cwd` 执行 `git rev-parse HEAD`，记为 `baseline = "git:<sha>"` |
| 节点 Agent 成功结束 | 相对该 SHA 采集 working tree + index 的 `git diff <sha>`，并附加 untracked 文件的合成 unified diff |
| 上传 | `PUT /v1/workflows/{id}/nodes/{nodeId}/diff`（Bearer；仅 Slave） |
| App 读取 | `GET` 同一路径（Bearer；只读） |

## 说明

- 基线是**节点开始时的 HEAD commit**，不是 stash。Agent 若在节点内 `commit`，diff 相对开始时的 SHA，仍能看到提交与未提交变更（`git diff <sha>` 对比工作区）。
- 仓库非 git 或 `rev-parse` 失败时：`baseline=null`，`files=[]`（仍可审核摘要/日志）。
- Diff 仅用于手机**只读渲染**；修改意见走 `revise`，不接受客户端提交的文件内容覆盖仓库。

## Revise（M05-P04）

| 时机 | 动作 |
|------|------|
| App `POST .../revise` | Gateway：仅 `awaiting_review` 允许；写入 `reviseHistory`；节点 → `running`；WS `workflow.revise` |
| Slave 收到 revise | 优先 `Agent.resume` + `agent.send(instruction)`，失败则新建 Agent 再跑；**沿用节点首次 baseline** |
| Agent 成功 | 再 `PUT` diff → 节点回 `awaiting_review` |
| 禁止 | revise 路径直接 `approved`；不写 `progress.md` |

样例响应见 `contracts/examples/node-diff.json`。
