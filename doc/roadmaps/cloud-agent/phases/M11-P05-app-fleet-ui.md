# M11-P05 — App 舰队管理 UI

## 目标

Flutter App 展示 Master 与其下 Slave：运行态、Start/Stop/Restart、添加/编辑/删除配置；进入工程仍走「选中 Slave → 其唯一 project」。

## 范围

**允许：** `app/lib/**`（API client + UI）、相关 widget 测试  

**禁止：** 在 App 内存或表单中保存 API Key；本机直接 SSH/进程管理绕过 Gateway

## 上下文

- [doc/slave-master.md](../../../slave-master.md) App UX
- 现有 Slave 列表 / 工程入口（M06）

## 完成定义（DoD）

- [ ] **舰队页**为配置/启停主入口（`/v1/masters`）；不以 `/v1/slaves` 作舰队主列表
- [ ] Slave 行：**process** + **gatewayOnline** + `lastError`
- [ ] Start / Stop / Restart；处理 409/503/504 与业务错误码文案
- [ ] 添加 / 编辑 / 删除（id 只读；repoId/cwd 冲突有提示）
- [ ] **工程入口**只列出 `running && gatewayOnline`
- [ ] 写配置/启停以 HTTP 成功 + 再 GET 刷新为准
- [ ] widget 或 API 单测覆盖解析与至少一条启停/CRUD 路径

## 禁止事项

- 把完整 token / API Key 打进日志或 UI
- 仅显示单一「online」而忽略三态
- 工程列表展示已 stopped 的 Slave 并可点进发任务
- 允许空 cwd 或不校验明显非法路径
