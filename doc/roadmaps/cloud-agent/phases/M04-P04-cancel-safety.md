# M04-P04 — 取消、错误分类与安全默认

## 目标

完善 cancel 路径、错误上报、cwd 越权拒绝；固化安全默认。

## 范围

**允许：** `slave/**`、必要时 gateway cancel 联调  

**禁止：** Workflow/DAG  

## 上下文

- SDK `run.supports("cancel")`
- architecture 安全表

## 完成定义（DoD）

- [ ] 取消指令到达时尝试 cancel；不支持则记录原因并尽快停接新 tool（尽力）
- [ ] repoId 不在白名单 → 拒绝执行并 error 事件
- [ ] API Key 仅来自环境变量名配置，不进日志
- [ ] README 安全注意事项一小节

## 禁止事项

- 信任 App 传来的任意 cwd 字符串而不校验白名单
