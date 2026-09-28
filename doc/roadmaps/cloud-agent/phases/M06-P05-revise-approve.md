# M06-P05 — 意见框 revise + 通过

## 目标

审核页输入框提交修改意见（revise）；「通过」调用 approve；通过后 UI 反映节点 approved 与下游推进。

## 范围

**允许：** `app/**`  

**禁止：** 在 App 写 progress.md  

## 上下文

- M05-P04 / M05-P05
- workflow 审核循环

## 完成定义（DoD）

- [x] 输入框非空可提交 revise，随后回到 running/日志
- [x] 通过按钮仅在 awaiting_review 可点
- [x] approve 成功后节点状态更新；可观察到进度变化（含 pull workflow）
- [x] reject 可选实现（若做了，需确认不写 progress）

## 禁止事项

- 跳过 Gateway 直连 Slave
