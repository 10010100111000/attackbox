# 直接编辑模型与全新启动

本次移除 `module_revisions`、`chapter_revisions`、`module_revision_chapters`、`chapter_revision_assets`，以及发布指针、封存标记、章节来源版本字段。当前结构与接口见 [数据模型](learning-platform-v1.md) 和 [接口契约](learning-platform-v1-flows.md)。

后台只区分未发布和已发布。编辑先留在浏览器，点击保存后提交；已发布内容保存后立即生效。不保留草稿副本或历史正文。删除真实删除内容；操作审计继续保留。

## 启动

迁移历史已重建为 `web/server/db/migrations/0000_simple_platform.sql`，不支持在旧表上执行升级。应创建空数据库，或停止应用后重置已有数据库。

在 `web/` 源码目录安装依赖并设置实际数据库连接：

```bash
export DATABASE_URL='postgres://USER:PASSWORD@HOST:5432/DATABASE'
npm ci
npm run db:migrate
npm run admin:init -- 'admin@example.com' 'YOUR_PASSWORD'
npm run build
node .output/server/index.mjs
```

`db:migrate` 可重复执行，使用 Drizzle 标准迁移记录，默认分类随基线插入。遇到旧内容表或旧迁移记录会拒绝执行，不会自动删除数据库。

如需清空已有数据库，先停止连接该库的应用，再显式指定待清空数据库名称：

```bash
RESET_DATABASE='DATABASE' npm run db:reset
npm run admin:init -- 'admin@example.com' 'YOUR_PASSWORD'
```

该操作删除目标库的 `public` 和 `drizzle` schema，重建全部账号、内容和迁移记录。数据库用户需拥有相应权限。也可在 VPS 指定 `WEB_DIR` 后运行 `scripts/reset_db_remote.sh`；脚本不自动停止或启动服务。

本次代码修改不执行线上数据库清理或部署。

## 验证

```bash
npm test
npx tsc --noEmit -p tsconfig.json
npm run build
```

不要在构建期间同时执行 TypeScript 检查，Nuxt 会重建 `.nuxt/tsconfig.json`。

隔离的本地测试库迁移并启动应用后：

```bash
CONTENT_MIGRATION_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55440/attackbox_schema_test' \
CONTENT_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55440/attackbox_content_test' \
CONTENT_TEST_BASE_URL='http://127.0.0.1:18884' npm run test:content
```

测试仅接受本机地址和以 `_test` 结尾的数据库名称。迁移测试检查空库基线、重放、分类种子以及旧表消失；生命周期测试覆盖权限、答案保留、规范化、并发冲突、原子回滚、直接更新、排序、新增与删除章节、下架、重新发布和真实删除。

原先针对草稿与修订历史的远程脚本测试已删除，由本地回归覆盖，避免误连远程数据库执行旧结构操作。
