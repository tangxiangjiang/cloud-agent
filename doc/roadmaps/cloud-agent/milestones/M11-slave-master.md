# M11 — Slave-Master（一工程一 Slave）

## 目标

引入 **slave-master 守护进程**：配置与启停多个 **1:1 工程** 的子 Slave；App 经 Gateway 查看配置、拉起/关闭/重启，并增删改 Slave 配置，降低多工程共进程时的故障影响面。

设计权威文档：[../../slave-master.md](../../slave-master.md)。

## 非目标

- 不做多机 Master / K8s  
- 不让 App 持有或填写 `CURSOR_API_KEY`  
- 不把 Task/Workflow 数据面改成「只经 Master 转发」（子 Slave 仍直连 Gateway）  
- 不在本里程碑删除 Gateway 对旧多工程 Slave 的兼容（可标记 deprecated）  

## 依赖

M03（Slave 出站 / Gateway 登记）、M04（Local Slave）、M06（App 主路径）；建议在 M08～M10 稳定后做。

## 验收

- Master **冷启动全停**；`enabled` 只控制是否允许 Start  
- 多条 Slave，每条一 project；repoId/cwd 唯一；共享一对 Gateway token  
- App **舰队页**主管理；工程入口仅 `running && gatewayOnline`  
- Start/Stop/Restart 同步等回执；stop 会尽力 cancel 再杀进程  
- 一子异常不影响其他；Master 重启收养、不双开  
- 迁移后主工程保留旧 slaveId，既有 Workflow/Chat 仍可用  

设计决议见 [slave-master.md §决议](../../../slave-master.md)。  



## Phases

| Phase | 文档 |
|-------|------|
| M11-P01 | [设计与配置契约](../phases/M11-P01-design-config.md) |
| M11-P02 | [Master 守护与子进程生命周期](../phases/M11-P02-master-lifecycle.md) |
| M11-P03 | [子 Slave 一工程约束与迁移](../phases/M11-P03-one-project-slave.md) |
| M11-P04 | [Gateway Master 通道与 HTTP](../phases/M11-P04-gateway-master-api.md) |
| M11-P05 | [App 舰队管理 UI](../phases/M11-P05-app-fleet-ui.md) |
| M11-P06 | [联调、审计与部署文档](../phases/M11-P06-docs-harden.md) |
