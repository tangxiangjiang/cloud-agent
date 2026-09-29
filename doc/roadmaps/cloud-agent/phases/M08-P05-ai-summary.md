# M08-P05 — 可选 AI 摘要

## 目标

在结构化采集之外，可选短跑 Local Agent（只读）生成给人看的 `summary` / 提示性 `inferredPhaseStatus`；失败则降级为规则摘要。

## 范围

**允许：** `slave/**`（Agent 调用、prompt）；文档说明

**禁止：** Agent 写盘、push、改 progress、改 Gateway 节点；阻塞无 Key 环境的同步主路径

## 上下文

- [doc/project-sync.md](../../../project-sync.md) AI 职责
- 现有 Local Agent / commit message AI 模式可参考（短超时）

## 完成定义（DoD）

- [ ] 有 API Key 且非 stub 时可生成 summary；超时/失败回退规则摘要
- [ ] prompt 明确禁止写文件与批准节点
- [ ] payload 仍含完整结构化字段；AI 仅附加文案
- [ ] 文档注明可关闭（env / config）

## 禁止事项

- 将整仓 diff 塞进模型上下文
- 无用户操作时自动「修复」Gateway 状态
