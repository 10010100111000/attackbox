# 学习平台直接编辑模型与部署验证

当前系统采用直接编辑模型。数据结构与接口规范请参阅 [数据模型规范](learning-platform-v1.md) 与 [接口与事务契约](learning-platform-v1-flows.md)。

## 架构原则

1. **直接编辑与实时生效**：后台内容只区分未发布（`unpublished`）和已发布（`published`）。编辑保留在前端表单，保存后原子写入数据库；选择“保存为草稿”将同时下架，选择“保存并发布”则在同一事务中校验并公开内容。
2. **单一记录与乐观锁**：不保留草稿副本或历史版本表。通过 `lock_version` 递增进行并发冲突检测，避免后提交覆盖。
3. **真实删除与安全审计**：删除模块或题目执行物理删除；关键变更记录于 `audit_logs`。

增量迁移 `0002_module_prerequisites_duration.sql` 添加前置模块关系和章节预计时长，已有内容保留；执行 `npm run db:migrate` 即可。

## 初始化与启动流程

增量迁移 `0001_module_outcomes.sql` 添加学习目标数组，默认空数组，保留已有内容。已有当前基线数据库只需执行 `npm run db:migrate`，无需重置。

迁移基线位于 `web/server/db/migrations/0000_simple_platform.sql`，适用于全新数据库部署。

在 `web/` 目录下配置实际数据库连接并执行部署：

```bash
export DATABASE_URL='postgres://USER:PASSWORD@HOST:5432/DATABASE'
npm ci
npm run db:migrate
npm run admin:init -- 'admin@example.com' 'YOUR_PASSWORD'
npm run build
node .output/server/index.mjs
```

- `npm run db:migrate`：安全应用 Drizzle 迁移，幂等执行，内置初始化分类。
- `npm run admin:init`：初始化管理员账号（角色设为 `admin`，状态设为 `active`）。

## 重置数据库

如需在全新部署或开发环境中彻底清空已有数据：

```bash
RESET_DATABASE='DATABASE' npm run db:reset
npm run admin:init -- 'admin@example.com' 'YOUR_PASSWORD'
```

该操作将清理 `public` 与 `drizzle` schema，并基于最新基线全量重建所有表结构与分类种子。

## 验证与测试套件

### 1. 本地单元测试

在 `web/` 目录下执行前端与服务端逻辑单元测试：

```bash
npm test
```

### 2. 数据库迁移与内容生命周期测试

使用隔离的本地测试数据库验证迁移基线与直接编辑生命周期：

```bash
CONTENT_MIGRATION_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55440/attackbox_schema_test' \
CONTENT_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55440/attackbox_content_test' \
CONTENT_TEST_BASE_URL='http://127.0.0.1:18884' npm run test:content
```

测试覆盖范围：
- 空库基线迁移、重放幂等性、默认分类种子插入。
- 权限隔离、答案 HMAC 计算与存储、输入规范化。
- `lock_version` 并发冲突拒绝、事务原子回滚。
- 章节排序编排、新增/删除章节、发布完整性校验、下架与物理删除。

### 3. 生产环境端到端认证测试

```bash
REMOTE_BASE_URL='http://77.67.1.106' node web/tests/auth-remote-e2e.mjs
```
