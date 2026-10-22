# AttackBox 学习平台 Phase 2：数据库基础与账号闭环设计规范

- 日期：2026-10-23
- 状态：已确认 (Approved)
- 依据：[AttackBox 学习平台数据库设计 v1](../../database/learning-platform-v1.md)、[Phase 2 实施计划](../../database/phase2-plan.md)

---

## 1. 目标与范围

根据 `docs/database/phase2-plan.md`，渐进式交付第 1 阶段（数据库与服务端基础）及第 2 阶段核心表：
1. 在 `web/` 内建立 Drizzle ORM 与 PostgreSQL 基础设施（配置 `drizzle.config.ts`、连接池、schema 导出）。
2. 实现账号体系核心数据表：`users`、`auth_sessions`、`email_tokens`、`audit_logs`。
3. 遵循严格的 PostgreSQL 17 数据库级约束（UUIDv4、邮箱小写与格式检查、Argon2id 密码字段、32 字节 bytea Token 摘要、状态/角色枚举检查、级联与审计保护）。
4. 在远程服务器 `ssh laji@77.67.1.106` 上配置专用 PostgreSQL 数据库，执行迁移生成与应用，并运行自动化测试套件验证数据完整性与约束正确性。

---

## 2. 数据库表结构定义 (Drizzle Schema)

### 2.1 users 表
长期身份表，记录学习平台用户基本凭证与状态。

- `id`: `uuid`，主键，应用生成 (`gen_random_uuid()`)
- `email`: `varchar(254)`，唯一索引，CHECK 约束 `email = lower(btrim(email))`
- `display_name`: `varchar(64)`，非空，CHECK 约束 `length(btrim(display_name)) > 0`
- `password_hash`: `text`，可为空（预留纯 OAuth 账号），存储 Argon2id 密码摘要
- `role`: `varchar(16)`，默认 `'user'`，CHECK 约束 `role in ('user', 'admin')`
- `status`: `varchar(16)`，默认 `'pending'`，CHECK 约束 `status in ('pending', 'active', 'suspended')`
- `email_verified_at`: `timestamptz`，可空
- `created_at`: `timestamptz`，非空，默认 `now()`
- `updated_at`: `timestamptz`，非空，默认 `now()`
- `last_login_at`: `timestamptz`，可空

**约束与安全策略**：
- 邮箱不区分大小写，唯一。
- 注册接口不可直接指定 `role` 与 `status`。第一位管理员通过受控初始化脚本或迁移种子设定。

### 2.2 auth_sessions 表
服务端会话凭据表，保存高熵 Session 随机串的 SHA-256 摘要（32 字节）。

- `id`: `uuid`，主键，应用生成
- `user_id`: `uuid`，外键引用 `users.id`，`ON DELETE CASCADE`
- `token_hash`: `bytea`，唯一索引，CHECK 约束 `octet_length(token_hash) = 32`
- `created_at`: `timestamptz`，非空，默认 `now()`
- `expires_at`: `timestamptz`，非空，CHECK 约束 `expires_at > created_at`
- `last_seen_at`: `timestamptz`，非空，默认 `now()`
- `revoked_at`: `timestamptz`，可空
- `ip`: `inet`，可空
- `user_agent`: `varchar(512)`，可空

**索引与生命周期**：
- 唯一索引：`token_hash`
- 复合索引：`(user_id, expires_at)`
- 会话默认 24 小时绝对有效期，保持登录可达 30 天，不使用无限滑动续期。

### 2.3 email_tokens 表
一次性邮件安全令牌（邮箱验证、密码找回），仅存储 SHA-256 摘要。

- `id`: `uuid`，主键，应用生成
- `user_id`: `uuid`，外键引用 `users.id`，`ON DELETE CASCADE`
- `target_email`: `varchar(254)`，非空
- `purpose`: `varchar(32)`，非空，CHECK 约束 `purpose in ('verify_email', 'reset_password')`
- `token_hash`: `bytea`，唯一索引，CHECK 约束 `octet_length(token_hash) = 32`
- `created_at`: `timestamptz`，非空，默认 `now()`
- `expires_at`: `timestamptz`，非空，CHECK 约束 `expires_at > created_at`
- `consumed_at`: `timestamptz`，可空

**业务约束**：
- 邮箱验证有效期 24 小时，密码重置有效期 30 分钟。
- 消费凭据与状态更新必须在同一事务内完成，防止双重消费（TOCTOU）。

### 2.4 audit_logs 表
关键审计日志表，记录封禁、角色变更、重要安全事件。

- `id`: `uuid`，主键，应用生成
- `actor_user_id`: `uuid`，外键引用 `users.id`，`ON DELETE RESTRICT`（防止删除用户导致审计记录丢失）
- `action`: `varchar(80)`，非空
- `target_type`: `varchar(40)`，非空
- `target_id`: `uuid`，可空
- `metadata`: `jsonb`，默认 `'{}'::jsonb`，白名单字段，严禁记录明文密码、Session 或 Token
- `request_id`: `uuid`，可空
- `ip`: `inet`，可空
- `created_at`: `timestamptz`，非空，默认 `now()`

**索引**：
- 复合索引：`(target_type, target_id, created_at DESC)`
- 复合索引：`(actor_user_id, created_at DESC)`

---

## 3. 代码架构与目录结构

在 `web/` 下建立：
```text
web/
├── drizzle.config.ts              # Drizzle 迁移配置
├── server/
│   └── db/
│       ├── index.ts               # 导出连接池 client 与 db 实例 (postgres.js)
│       ├── schema/
│       │   ├── auth.ts            # users, auth_sessions, email_tokens
│       │   ├── audit.ts           # audit_logs
│       │   └── index.ts           # 统一汇聚导出
│       └── migrations/            # 由 drizzle-kit generate 生成的纯 SQL 迁移产物
└── tests/
    └── db-remote-verify.mjs       # 远程数据库自动化测试套件
```

---

## 4. 远程验证流程 (ssh laji@77.67.1.106)

1. **远程 PostgreSQL 准备**：
   - 远程已安装 PostgreSQL 17.11。
   - 创建数据库 `attackbox_db`（正式数据环境）和 `attackbox_test`（独立验证与测试环境）。
   - 配置专用角色 `attackbox` 并授权。
2. **打包与同步**：
   - 依赖文件：在 `web/package.json` 中配置 `drizzle-orm`, `drizzle-kit`, `postgres`。
   - 生成的 SQL 迁移文件与测试脚本上传至远程主机。
3. **远程自动化测试套件 (`db-remote-verify.mjs`) 验证项**：
   - **连接性检查**：成功连接远程 PostgreSQL 17。
   - **迁移应用**：执行 Drizzle 迁移，检查表、索引、约束是否存在。
   - **约束有效性检查**：
     - 尝试插入大写或带首尾空格的 email，验证是否被 check 约束拦截或标准化。
     - 插入重复 email，验证唯一冲突。
     - 插入非法 token_hash（字节长度不为 32），验证是否被拒绝。
     - 插入非法 role/status/purpose 枚举值，验证 CHECK 约束生效。
   - **外键级联检查**：
     - 插入 user 及关联 session、email_token，删除 user 时 session/token 自动级联删除。
     - 插入 audit_log，删除 actor user 时必须抛出外键限制错误 (`ON DELETE RESTRICT`)。
4. **输出验证报告并记录**。
