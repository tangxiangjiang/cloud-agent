# M11-P06 — 联调、审计与部署文档

## 目标

端到端联调 Master ↔ Gateway ↔ App ↔ 双子 Slave；补齐 deploy / 审计说明；标明旧多工程单进程废弃路径。

## 范围

**允许：** `doc/deploy.md`、`doc/architecture.md`、`gateway/docs/audit.md`、联调清单、轻微 bugfix  

**禁止：** 新增大功能；无确认时删除旧 Slave 单进程入口

## 上下文

- [doc/slave-master.md](../../../slave-master.md)
- [mvp-checklist.md](../mvp-checklist.md)（可增「舰队」小节）

## 完成定义（DoD）

- [ ] deploy：装 Master、示例配置、拆旧配置、**冷启动需 App Start**、可选 OS 自启 Master
- [ ] 说明共享 Cursor Key 与 `maxRunningSlaves` 的关系
- [ ] 审计事件表；错误码表；联调无 Key 进日志
- [ ] 双工程：停 A 不影响 B；Master 重启收养不双开
- [ ] roadmap / architecture 指向 M11；旧 multi-project 标 deprecated
- [ ] mvp-checklist 舰队相关勾选已存在则核对通过

## 禁止事项

- 引导用户把 Key 提交进 git
- 未文档说明就默认关掉旧单进程启动方式
- 暗示 `enabled=true` 即开机自动跑 Agent
