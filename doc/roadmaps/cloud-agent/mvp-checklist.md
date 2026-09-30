# MVP 完成检查清单

对照实现勾选（人工或联调后）。权威运行进度见 [progress.md](./progress.md)；部署步骤见 [../../deploy.md](../../deploy.md)。

## 配对

- [ ] Gateway 启动并打印 / 固定 pair code
- [ ] `POST /v1/auth/pair` 返回 Bearer token
- [ ] App 输入 Gateway URL + pair code 可登录
- [ ] Token 在 `flutter_secure_storage`；未登录不能进主页
- [ ] 日志无完整 token / 无 `CURSOR_API_KEY`

## 单任务

- [ ] `POST /v1/tasks` → Slave 收到 `task.assign`（或 stub）
- [ ] App/WS 可见 `assistant.delta` / `done`（或 HTTP `events?afterSeq=`）
- [ ] `POST /v1/tasks/{id}/cancel` 尽力取消

## DAG 工作流

- [ ] `POST /v1/workflows` 加载样例（`examples/sample-dag.json`）
- [ ] App 列表可见 workflow id/状态；详情可见节点
- [ ] `POST .../start` → Slave 串行跑 ready 节点
- [ ] Agent 成功 → `awaiting_review`（不直接 approved）
- [ ] 下游在依赖未 approve 前保持 pending

## Diff

- [ ] 节点 `awaiting_review` 时 `GET .../diff` 有文件列表
- [ ] App 只读 diff；无编辑/保存/apply-patch

## Revise

- [ ] `POST .../revise` 仅 awaiting_review；instruction 进 `reviseHistory`
- [ ] Slave follow-up / 再跑 → 刷新 diff → 再 awaiting_review
- [ ] revise **不**写 progress

## Approve / Progress

- [ ] `POST .../review` decision=approve → 节点 approved，下游 ready
- [ ] Slave **仅 approve** 时更新 `progressDoc`（basename=`progress.md`）
- [ ] reject → 不写 progress；下游不解锁

## 硬化

- [ ] 审计：`GET /v1/audit` 或 `GATEWAY_AUDIT_LOG` JSONL
- [ ] pair / revise 超限返回 429
- [ ] Local only；未引导提交密钥进 git

## Slave-Master（M11）

- [ ] Master 配置多条 Slave，每条仅一工程
- [ ] App 可见配置与 running/stopped；Start / Stop / Restart
- [ ] App 增删改配置经 Gateway → Master 落盘
- [ ] 一子 Slave 崩溃不影响其他工程
