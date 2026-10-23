# 学习平台数据库规范与文档

当前系统采用直接编辑模型（Direct Editing Model），不保留草稿副本、修订历史或发布快照。管理员选择“保存为草稿”或“保存并发布”，单次事务同时保存内容与公开状态；保存为草稿会下架已发布模块，并通过 `lock_version` 乐观锁保障并发安全。

## 核心文档导航

- [数据模型规范](learning-platform-v1.md)：11 张核心数据表结构、字段约束与设计原理。
- [接口与事务契约](learning-platform-v1-flows.md)：管理后台与学习端 API 规范、请求响应结构与事务原子性约定。
- [部署与验证运维指南](content-lifecycle-repairs.md)：数据库初始化、重置、迁移与自动化测试执行流程。

## 迁移与启动基线

迁移基线为 `web/server/db/migrations/0000_simple_platform.sql`，仅用于空数据库。新环境部署直接执行 `npm run db:migrate` 与 `npm run admin:init` 初始化。之后的增量迁移也由 `npm run db:migrate` 执行。
