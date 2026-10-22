# 数据库 Phase 2 账号闭环实现计划 (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 根据 `docs/database/phase2-plan.md` 与 `learning-platform-v1.md`，构建基于 Nuxt Nitro 与 Drizzle ORM (PostgreSQL) 的数据库基础设施，实现账号闭环及审计日志的核心数据表（`users`、`auth_sessions`、`email_tokens`、`audit_logs`），并在远程服务器 `ssh laji@77.67.1.106` 的 PostgreSQL 17 上执行完整的自动化迁移与测试验证。

**Architecture:** 在 `web/server/db` 中使用 Drizzle ORM 定义结构体、索引与检查约束；使用 `drizzle-kit` 自动生成版本化 SQL 迁移文件；通过针对远程 PostgreSQL 的自动化测试脚本 `web/tests/db-remote-verify.mjs` 在远程测试库 `attackbox_test` 执行建表、迁移、约束生效（大小写邮箱、32字节Token摘要、枚举）、级联清理与审计拦截测试。

**Architecture Diagram:**

```mermaid
graph TD
    subgraph "Local Workspace (web/)"
        A[schema/auth.ts<br/>users, sessions, tokens] --> C[schema/index.ts]
        B[schema/audit.ts<br/>audit_logs] --> C
        C --> D[drizzle.config.ts]
        D --> E[drizzle-kit generate<br/>SQL Migrations]
        F[tests/db-remote-verify.mjs]
    end

    subgraph "Remote Server (laji@77.67.1.106)"
        G[PostgreSQL 17.11]
        G --> H[(attackbox_test)]
        G --> I[(attackbox_db)]
        E -->|SCP / Synchronize| J[/home/laji/attackbox_db_verify]
        F -->|Run via Node 20| J
        J -->|Execute & Assert| H
    end
```

**Tech Stack:** 
- Nuxt 3 (Nitro)
- TypeScript 5.7
- Drizzle ORM 0.38+ / Drizzle Kit
- PostgreSQL 17.11 (`postgres.js` 驱动)
- Remote SSH / Bash / Node.js test runner

## Global Constraints

- 所有业务主键必须使用 UUIDv4 (`defaultRandom()`)，不使用自增 ID。
- 时间戳字段一律采用带时区的 `timestamptz`，默认存取 UTC。
- `email` 必须有 CHECK 约束：`email = lower(btrim(email))`。
- `token_hash` 必须为 bytea 且长度严格等于 32 字节。
- `auth_sessions` 与 `email_tokens` 的外键引用 `users.id` 使用 `ON DELETE CASCADE`。
- `audit_logs.actor_user_id` 的外键引用 `users.id` 使用 `ON DELETE RESTRICT`。
- 所有的验证、迁移执行和测试必须在远程服务器 `ssh laji@77.67.1.106` 上运行。

---

### Task 1: 在 web/ 配置 Drizzle ORM 依赖与环境

**Files:**
- Modify: `web/package.json`
- Create: `web/drizzle.config.ts`
- Create: `web/server/db/index.ts`

**Interfaces:**
- Consumes: `process.env.DATABASE_URL`
- Produces: `db` 实例 (Drizzle PostgresJsDatabase) 与 `client` (postgres.Sql)

- [ ] **Step 1: 更新 package.json 声明 drizzle-orm, drizzle-kit 与 postgres**

在 `web/package.json` 中添加 `drizzle-orm` 与 `postgres` 到 `dependencies`，`drizzle-kit` 到 `devDependencies`，并增加 `db:generate` 脚本。

- [ ] **Step 2: 创建 drizzle.config.ts**

配置 Drizzle Kit 读取 `server/db/schema/index.ts` 并将 SQL 迁移输出到 `server/db/migrations`。

- [ ] **Step 3: 创建 server/db/index.ts 数据库客户端导出**

实现连接池管理，支持通过 `DATABASE_URL` 初始化 `postgres.js` 驱动并包装为 Drizzle `db` 客户端。

---

### Task 2: 实现账号体系与审计 Schema 定义

**Files:**
- Create: `web/server/db/schema/types.ts`
- Create: `web/server/db/schema/auth.ts`
- Create: `web/server/db/schema/audit.ts`
- Create: `web/server/db/schema/index.ts`

**Interfaces:**
- Consumes: `pgTable`, `uuid`, `varchar`, `timestamp`, `customType`, `check`, `uniqueIndex`, `index`
- Produces: `users`, `authSessions`, `emailTokens`, `auditLogs` 数据表对象

- [ ] **Step 1: 创建自定义字段类型 (types.ts)**

实现 `bytea`（存储 32 字节 Hash）与 `inet`（存储 IP 地址）自定义类型。

- [ ] **Step 2: 实现 auth.ts 数据表**

定义 `users`、`auth_sessions`、`email_tokens` 表，并设置 `check` 约束（email 格式与 lower、display_name、role、status、token_hash 长度 32 字节、expires_at > created_at、purpose）和级联规则。

- [ ] **Step 3: 实现 audit.ts 数据表**

定义 `audit_logs` 表，外键引用 `users.id` (RESTRICT)，定义 `(target_type, target_id, created_at DESC)` 及 `(actor_user_id, created_at DESC)` 索引。

- [ ] **Step 4: 实现 index.ts 统一汇聚导出**

统一导出全部表及约束。

---

### Task 3: 生成 SQL 迁移并编写迁移执行器

**Files:**
- Create: `web/server/db/migrate.ts`
- Generate: `web/server/db/migrations/*.sql`
- Generate: `web/server/db/migrations/meta/*`

**Interfaces:**
- Consumes: `drizzle-kit generate`
- Produces: 离线版本化 SQL 迁移文件，及可独立运行的 `migrate.ts` 迁移应用函数

- [ ] **Step 1: 运行 drizzle-kit generate 生成标准 SQL 迁移文件**

生成完整的 `0000_...sql` 迁移，包含所有建表、索引与 CHECK 约束。

- [ ] **Step 2: 检查生成的 SQL 确保覆盖全部 CHECK 约束与外键规则**

核验生成的 SQL 中包含 `octet_length(token_hash) = 32`、`email = lower(btrim(email))`、`ON DELETE CASCADE` 与 `ON DELETE RESTRICT`。

- [ ] **Step 3: 创建 migrate.ts 迁移应用脚本**

使用 `drizzle-orm/postgres-js/migrator` 编写迁移执行函数，支持传入目标连接字符串并自动应用所有未执行的迁移。

---

### Task 4: 远程 PostgreSQL 17 数据库及用户初始化

**Files:**
- Remote: `ssh laji@77.67.1.106`
- Script: `scripts/init_remote_postgres.sh`

**Interfaces:**
- Consumes: PostgreSQL 17 权限 (sudo -u postgres)
- Produces: 远程用户 `attackbox`、生产数据库 `attackbox_db`、测试数据库 `attackbox_test`

- [ ] **Step 1: 编写远程初始化数据库脚本 init_remote_postgres.sh**

创建用户 `attackbox`（配置高熵密码），创建数据库 `attackbox_db` 与 `attackbox_test`，设置所有者与连接权限。

- [ ] **Step 2: 在远程服务器执行初始化脚本并验证连接**

远程执行脚本，使用 `psql -U attackbox -d attackbox_test` 确认连接成功。

---

### Task 5: 编写远程自动化测试验证套件

**Files:**
- Create: `web/tests/db-remote-verify.mjs`

**Interfaces:**
- Consumes: `DATABASE_URL`（指向远程 `attackbox_test`）
- Produces: 自动化断言结果与详细验证日志

- [ ] **Step 1: 编写自动化测试脚本结构**

引入 `node:test` 与 `node:assert`，定义测试用例集：
1. 迁移执行断言：调用迁移器，确保所有表创建成功。
2. 约束测试：
   - 插入大写邮箱，验证被 CHECK 拒绝。
   - 插入重复邮箱，验证唯一索引冲突。
   - 插入非 32 字节 bytea token_hash，验证被 CHECK 拒绝。
   - 插入非法 role/status，验证被 CHECK 拒绝。
3. 级联测试：
   - 插入 user 并建立 session 和 email_token。
   - 删除 user，断言 session 与 email_token 被自动清理 (`CASCADE`)。
4. 审计保护测试：
   - 插入 user 与对应 audit_log。
   - 尝试删除该 user，断言操作因 `RESTRICT` 失败抛出外键违规异常。

---

### Task 6: 打包、上传至远程服务器并执行验证测试

**Files:**
- Create: `scripts/run_remote_db_test.ps1`

**Interfaces:**
- Consumes: 本地迁移与测试文件、SSH 凭据
- Produces: 远程完整测试运行输出与测试报告

- [ ] **Step 1: 编写打包与执行脚本 run_remote_db_test.ps1**

将 `web/` 下的迁移文件、测试套件及依赖声明打包成 tar 归档，通过 `scp` 发送到 `laji@77.67.1.106`。

- [ ] **Step 2: 在远程安装依赖并执行测试套件**

在远程执行 `node tests/db-remote-verify.mjs`，捕获所有用例运行状态。

- [ ] **Step 3: 提取远程测试结果并生成测试报告**

验证全部用例 100% 通过后，记录结果至文档。
