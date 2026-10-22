# AttackBox 学习平台 Phase 2：真实账号体系与 Nitro 服务端认证设计规范

- 日期：2026-10-23
- 状态：已确认 (Approved)
- 依据：[AttackBox 学习平台数据库设计 v1](../../database/learning-platform-v1.md)、[业务事务与接口契约](../../database/learning-platform-v1-flows.md)、[Phase 2 实施计划](../../database/phase2-plan.md)

---

## 1. 目标与范围

将 `web/` 的认证中心（登录、注册、密码找回）接入真实服务端与数据库：
1. 实现基于 Argon2id 的密码安全哈希与验证。
2. 实现基于 HttpOnly Cookie 与 32 字节高熵随机串的会话凭证机制（数据库仅存储 SHA-256 摘要）。
3. 遵循严格的账号状态生命周期（`pending` 注册 → 邮件 Token 激活 → `active` 登录；`suspended` 封禁拒绝）。
4. 在 Nuxt Nitro 中提供完整的认证 API：
   - `POST /api/auth/register`
   - `POST /api/auth/verify-email`
   - `POST /api/auth/login`
   - `POST /api/auth/logout`
   - `GET /api/auth/me`
   - `POST /api/auth/forgot-password`
   - `POST /api/auth/reset-password`
5. 提供受控的管理员初始化命令行工具 (`server/scripts/init-admin.mjs`)。
6. 前端接入 `composables/useAuth.ts`，驱动表单交互并替换原型预览。
7. 在远程服务器 `ssh laji@77.67.1.106` 上配置 Nitro 服务运行、Nginx `/api/auth/` 代理并执行端到端自动化测试验证。

---

## 2. 核心架构与凭据安全设计

### 2.1 密码哈希 (Argon2id)
- 使用 `argon2` 库的 Argon2id 算法，配置兼顾安全与性能的参数：
  - `type: argon2.argon2id`
  - `memoryCost: 65536` (64 MB)
  - `timeCost: 3`
  - `parallelism: 1`
- 密码比对时使用恒定时间比较，防止时序侧信道攻击。

### 2.2 会话管理 (Session Management)
- **Token 生成**：使用 `crypto.randomBytes(32).toString('hex')`（64 字符高熵不透明随机串）。
- **数据库摘要**：服务端使用 `crypto.createHash('sha256').update(rawToken).digest()` 计算 32 字节 bytea 存入 `auth_sessions.token_hash`。
- **Cookie 规范**：
  - 名称：`attackbox_session`
  - 属性：`httpOnly: true, secure: true, sameSite: 'lax', path: '/'`
  - 有效期：常规登录 24 小时 (`maxAge: 86400`)；勾选记住我 30 天 (`maxAge: 2592000`)。
- **授权判定**：
  - 每次读取根据 Cookie 的 SHA-256 摘要查表，要求 `expires_at > now()` 且 `revoked_at IS NULL`。
  - 要求关联用户 `status = 'active'`（`suspended` 用户即使持有旧 Cookie 也立即返回 401 并清理）。
  - 更新会话 `last_seen_at = now()`。

### 2.3 邮箱与安全令牌 (Email Tokens)
- 邮箱验证 Token 有效期 24 小时；密码重置 Token 有效期 30 分钟。
- 数据库只存储 SHA-256 摘要，消费（激活或重置）与用户状态变更在单事务内完成，防止双重消费。
- 找回密码接口返回通用成功提示，防止通过接口响应差异探测邮箱是否存在。

---

## 3. Nitro API 接口契约

### 3.1 `POST /api/auth/register`
- **请求体**：
  ```json
  {
    "email": "user@example.com",
    "password": "Password123",
    "displayName": "CyberWarrior"
  }
  ```
- **校验**：
  - `email`: 规范化为 `email.trim().toLowerCase()`，符合标准邮箱正则。
  - `password`: 长度 >= 6 位。
  - `displayName`: 默认取邮箱前缀或用户输入。
  - `role` / `status`: 接口禁止客户端提交，强制设为 `'user'` 与 `'pending'`。
- **业务逻辑**：
  - 检查邮箱是否已被占用；若已被 active 用户占用返回 409。
  - 计算 Argon2id 哈希，插入 `users`。
  - 生成 32 字节 verify_email 令牌，保存摘要至 `email_tokens`。
  - 触发 Mailer 输出激活链接（测试环境直接在响应带出 `verifyToken`）。
- **返回**：`201 Created`，`{ "message": "注册成功，请查收验证邮件", "verifyToken": "..." }`

### 3.2 `POST /api/auth/verify-email`
- **请求体**：`{ "token": "..." }`
- **业务逻辑**：
  - 计算 SHA-256 摘要，查 `email_tokens`。
  - 校验 `purpose = 'verify_email'`、`expires_at > now()`、`consumed_at IS NULL`。
  - 事务内更新 `users.status = 'active'`, `users.email_verified_at = now()`, `email_tokens.consumed_at = now()`。
- **返回**：`200 OK`，`{ "message": "邮箱验证成功，账号已激活" }`

### 3.3 `POST /api/auth/login`
- **请求体**：`{ "email": "user@example.com", "password": "Password123", "rememberMe": false }`
- **业务逻辑**：
  - 查用户；若不存在或密码错误返回 401（统一提示“邮箱或密码错误”，防枚举）。
  - 检查状态：`pending` 返回 403 `email_not_verified`；`suspended` 返回 403 `account_suspended`。
  - 密码验证通过，生成 Session Token 并写入 `auth_sessions`。
  - 设置 HttpOnly Cookie，更新用户 `last_login_at`。
- **返回**：`200 OK`，`{ "user": { "id": "...", "email": "...", "displayName": "...", "role": "user", "status": "active" } }`

### 3.4 `GET /api/auth/me`
- **请求**：从 Cookie 读取 `attackbox_session`。
- **业务逻辑**：核验 Session 与用户状态，更新 `last_seen_at`。
- **返回**：`200 OK`，`{ "user": { ... } }`；未登录或失效返回 401。

### 3.5 `POST /api/auth/logout`
- **请求**：从 Cookie 读取 Session。
- **业务逻辑**：将该 Session `revoked_at` 设为 `now()`，清除 Cookie。
- **返回**：`200 OK`，`{ "message": "已退出登录" }`

### 3.6 `POST /api/auth/forgot-password` & `POST /api/auth/reset-password`
- **找回密码**：若用户存在且为 active，生成 30 分钟有效的 `reset_password` 令牌；无论用户是否存在均返回统一提示。
- **重置密码**：验证重置 Token，更新用户 `password_hash`，并在同一事务中将该用户所有旧 Session 全部设为 `revoked_at = now()`（撤销所有已登录设备）。

---

## 4. 远程架构与协同部署 (77.67.1.106)

1. **Nitro 服务部署**：
   - 运行环境：Node 20。
   - 进程托管：通过 systemd (`attackbox-web.service`) 运行 `node .output/server/index.mjs`，监听本地 `127.0.0.1:3000`。
   - 环境变量：注入 `DATABASE_URL=postgres://attackbox:...@127.0.0.1:5432/attackbox_db` 与 `PORT=3000`。
2. **Nginx 路由划分**：
   - `location /api/auth/` -> 代理到 `http://127.0.0.1:3000/api/auth/`。
   - `location /api/` -> 保持代理至 Go Control (`http://127.0.0.1:18080`)。
   - `location /` -> 仍由 Nginx 提供高性能静态缓存与 SSR 回退。
