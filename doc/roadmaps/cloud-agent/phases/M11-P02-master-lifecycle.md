# M11-P02 — Master 守护与子进程生命周期

## 目标

实现本机 **slave-master** 进程：加载配置、按条 spawn/stop/restart 子 Slave、维护运行态（含崩溃感知），配置变更落盘。

## 范围

**允许：** `slave/` 下新入口（如 `master.ts` / `master/`）、master 配置读写、子进程管理、本地日志  

**禁止：** Gateway HTTP（P04）、App UI（P05）；本 phase 可用 CLI 验收

## 上下文

- [doc/slave-master.md](../../../slave-master.md) **决议**（权威源、stop、收养、配额、串行队列）
- 现有 `slave/src/index.ts` 启动参数

## 完成定义（DoD）

- [x] `slave-master` 可独立启动；**冷启动不自动 start**（`enabled` 只表示允许 Start）
- [x] 状态目录在 **`<masterRoot>/.local/slaves/<id>/`**（pid、config、logs）
- [x] 按 `slaveId` **串行**；`requestId` 幂等；`maxRunningSlaves` / `maxConcurrentStarts`
- [x] `stop`：可回调/请求 Gateway cancel → grace → kill（见设计决议 §6）
- [x] pid **收养**；禁止同 id 双开（停新保旧）
- [x] `project.id` / `cwd` 唯一校验；错误码对齐设计 §14
- [x] CRUD 写盘为权威；手改配置后可全量 report
- [x] 单测：冷启动全停；双假进程隔离；收养不重复 spawn
- [x] 文档：本机 CLI 用法（`npm run master -- …`）

## 禁止事项

- `enabled=true` 时开机自动拉起全部子进程
- 子进程共享同一 Agent 全局单例导致串台
- 在 Master 内直接调用 `@cursor/sdk` 跑任务
- 无 pid reconcile 就盲目二次 spawn
- 把 `.local/slaves` 写进各业务 git 仓库根目录
