# M06-P02 — 工作流列表/详情与触发

## 目标

实现工作流列表与详情：展示节点状态；提供「开始」触发；轮询或后续 WS 刷新状态。

## 范围

**允许：** `app/**` + 调用已有 Gateway workflow API  

**禁止：** diff 页、revise（下 phase）  

## 上下文

- M05 workflow API
- flutter-app 页面表

## 完成定义（DoD）

- [x] 列表展示 workflow id/状态
- [x] 详情展示各节点状态（含 awaiting_review 高亮）
- [x] 可触发 start
- [x] 错误态可提示

## 禁止事项

- 在 App 编辑 DAG JSON
