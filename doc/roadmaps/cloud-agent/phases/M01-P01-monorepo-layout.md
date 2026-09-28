# M01-P01 — 仓库目录与根 README

## 目标

创建 monorepo 顶层目录与根 README，标明各子项目职责与文档入口。

## 范围

**允许：**

- 根 `README.md`
- 空目录或占位：`gateway/`、`slave/`、`app/`、`contracts/`
- 必要时 `.gitignore` 基础项

**禁止：**

- 实现业务代码
- 修改 `doc/` 下架构结论（除非修正死链）

## 上下文

- [doc/architecture.md](../../../../architecture.md)
- [doc/roadmaps/cloud-agent/README.md](../README.md)

## 完成定义（DoD）

- [x] 上述目录存在
- [x] 根 README 说明 Go Gateway / Node Slave / Flutter App / contracts / doc
- [x] 链接到 `doc/README.md` 与本 roadmap

## 禁止事项

- 引入 Cloud Agent 相关依赖或目录暗示云端整仓执行
