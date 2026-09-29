# M09-P03 — Ask / Plan 与模型列表

## 目标

顶栏切换 Agent / Ask / Plan；模型下拉（Auto + 列表）；Slave 按 mode 注入系统前缀（Ask/Plan 偏只读）。

## 范围

**允许：** `app/**`、`slave/**`（prompt / 可选工具限制）、`GET /v1/models` 或等价

## 完成定义（DoD）

- [ ] 三种模式 UI + 请求字段
- [ ] Auto 与显式 model id
- [ ] Ask/Plan 系统提示文档化且生效（单测或集成可观察）
- [ ] 模型列表来自 Gateway（可配置静态表）

## 禁止事项

- Ask 模式仍默认可任意写仓且无任何提示/限制
