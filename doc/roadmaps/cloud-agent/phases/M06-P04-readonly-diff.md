# M06-P04 — 只读 diff 视图

## 目标

审核态展示只读可视 diff：文件列表 + patch 渲染；不可编辑源码。

## 范围

**允许：** `app/**`；可引入只读 diff 渲染库  

**禁止：** 编辑/保存文件；apply patch  

## 上下文

- M05-P03 diff API
- flutter-app 审核页

## 完成定义（DoD）

- [ ] awaiting_review 可打开 diff
- [ ] 多文件可切换
- [ ] UI 无编辑控件改代码内容
- [ ] 加载失败有错误提示

## 禁止事项

- TextField 绑到文件全文并提供「保存到仓库」
