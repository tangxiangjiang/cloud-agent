# M07-P02 — 样例 DAG 与部署文档

## 目标

提供可加载的样例 DAG（指向本 roadmaps phases），以及本机部署 Gateway + Slave + App 联调说明。

## 范围

**允许：** `doc/**`、`examples/**` 或 `workflow/` 样例、各子项目 README 补强  

**禁止：** 新增大功能代码（仅胶水与文档）  

## 上下文

- 全 milestone 已实现前提
- [doc/workflow.md](../../../../workflow.md)

## 完成定义（DoD）

- [ ] 样例 DAG JSON 可被 Gateway/Slave 加载
- [ ] 部署文档含：环境变量、配对、启动顺序、常见失败
- [ ] 明确 Local only / 禁止 Cloud

## 禁止事项

- 引导用户把密钥提交进 git
