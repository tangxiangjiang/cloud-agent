# M10-P02 — App 节点策略控件

## 目标

工作流详情每个任务行：模型下拉（默认 Auto）、「自动通过」「自动开始下个任务」Switch（默认关）；ready 节点可手动开始。

## 范围

**允许：** `app/**`；调用 PATCH / start / continue

## 完成定义（DoD）

- [ ] UI 形态为下拉 + Switch（默认关）
- [ ] 修改即 PATCH 到 Gateway
- [ ] ready + 未自动开跑时显示开始
- [ ] widget 测试覆盖默认关与开关展示

## 禁止事项

- 在 App 存 CURSOR_API_KEY
- 开关默认值为开
