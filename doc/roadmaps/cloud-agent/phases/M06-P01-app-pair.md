# M06-P01 — Flutter 工程与配对登录

## 目标

创建 Flutter 应用：配置 Gateway URL、配对换 token、安全存储、基础导航壳。

## 范围

**允许：** `app/**`  

**禁止：** 工作流业务页（可占位）；内置 CURSOR_API_KEY  

## 上下文

- [doc/flutter-app.md](../../../../flutter-app.md)
- Gateway pair API
- [app/README.md](../../../../app/README.md)

## 完成定义（DoD）

- [x] 可输入 gateway base URL 与 pair code 登录
- [x] token 存 `flutter_secure_storage`
- [x] 未登录不能进主页
- [x] README 说明 Flutter 版本与运行方式

## 禁止事项

- 明文把 token 写进源码或日志完整打印
