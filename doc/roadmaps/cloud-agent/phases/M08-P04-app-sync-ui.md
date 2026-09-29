# M08-P04 — App 工程页同步 UI

## 目标

在 Flutter「Slave → 工程」路径提供 **同步** 按钮与结果展示（最近同步时间、摘要、warnings）；不引入独立 Workflows 入口。

## 范围

**允许：** `app/**`；调用已有 Gateway sync API

**禁止：** 在 App 编辑 progress / DAG；持有 CURSOR_API_KEY

## 上下文

- [doc/flutter-app.md](../../../flutter-app.md)
- [doc/project-sync.md](../../../project-sync.md) App UI 节
- 现有 `ProjectListPage` / `SlavesApi`

## 完成定义（DoD）

- [x] 工程列表或工程详情可触发同步；Slave offline 时禁用并提示
- [x] loading / 成功 / 失败反馈
- [x] 展示 `syncedAt`、branch、dirty、summary、warnings 列表
- [x] warnings 可引导「继续当前 Workflow」（若报告带 workflowId）
- [x] widget/API 单测覆盖主路径

## 禁止事项

- 同步失败时静默新建 Workflow
- 在 UI 暴露 Gateway token 全文
