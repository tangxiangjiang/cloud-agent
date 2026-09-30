# M11-P03 — 子 Slave 一工程约束与迁移

## 目标

子 Slave **强制** 仅一个 project；提供从旧多 `projects[]` 配置拆到 Master `slaves[]` 的迁移说明或脚本。

## 范围

**允许：** `slave/src/config.ts` 校验、启动失败信息、迁移脚本/文档、`config.example.yaml` 更新  

**禁止：** 删除 Gateway 对「已 online 的旧多工程 Slave」的兼容（可 warn）

## 上下文

- [doc/slave-master.md](../../../slave-master.md) 迁移节
- [doc/deploy.md](../../../deploy.md)

## 完成定义（DoD）

- [ ] 子模式：`projects.length !== 1` → 明确报错退出（或 `--legacy-multi-project` 才允许多个，并打 deprecated 日志）
- [ ] Master 生成的子配置始终为单 project；**无**每条独立 `tokenEnv`（共享 Master `tokenEnv`）
- [ ] 迁移脚本/文档：**主工程（或指定 primary）继承旧 `slaveId`**；其余 `slave_<repoId>`；输出映射表
- [ ] 迁移结果满足同 Master 下 **repoId / cwd 唯一**
- [ ] 验收说明：迁移后主工程既有 Workflow 的 `slaveId` 仍能命中
- [ ] example 配置展示 1:1 形态（含 `allowedRoots` / `maxRunningSlaves` / `maxConcurrentStarts`）
- [ ] 单测：多 project 无 legacy 标志时拒绝加载

## 禁止事项

- 静默忽略多余 project（必须失败或明确 legacy）
- 迁移时给所有工程都换新 id 导致旧 workflow 全部失效（除非用户显式选择）
- 迁移脚本改写用户仓库业务代码
