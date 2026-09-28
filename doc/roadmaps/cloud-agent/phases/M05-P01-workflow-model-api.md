# M05-P01 — Workflow/DAG 模型与 API

## 目标

Gateway 增加 workflow 资源：创建/加载 DAG、查询状态、节点列表；与 contracts 一致。

## 范围

**允许：** `gateway/**`、`contracts/**` 微调  

**禁止：** Flutter；Slave 调度实现（可只存状态）  

## 上下文

- [doc/workflow.md](../../../../workflow.md)
- M01-P03 contracts

## 完成定义（DoD）

- [ ] 可 POST/GET workflow（含 nodes、dependsOn）
- [ ] 节点状态字段完整
- [ ] 非法环检测或文档声明「首版仅支持已校验 DAG」
- [ ] 鉴权保护

## 禁止事项

- approve 自动写 progress（属 M05-P05）
