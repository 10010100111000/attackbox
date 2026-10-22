# 真实账号体系与 Nitro 服务端认证实现计划 (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 Nuxt Nitro 与 PostgreSQL 构建真实的账号认证体系，实现 Argon2id 密码哈希、Session Cookie 会话管理、7 个完整的 Auth API、受控管理员 CLI、前端表单对接，并在远程服务器 `ssh laji@77.67.1.106` 上部署 Node/Nitro 常驻服务、配置 Nginx 代理与执行端到端自动化验证。

**Architecture:** 服务端在 `web/server/utils/auth.ts` 中封装 Argon2id 密码加密与 32 字节 SHA-256 Token 摘要计算；在 `web/server/api/auth/*` 实现注册、邮箱验证、登录、登出、身份核验、找回与重置密码；前端通过 `web/composables/useAuth.ts` 接入真实网络请求与状态管理；远程服务器使用 systemd 托管 Node 3000 服务，Nginx 将 `/api/auth/` 反代至 Nitro 服务。

**Architecture Diagram:**

```mermaid
graph TD
    subgraph "Client Layer (Browser)"
        A[LoginForm.vue / RegisterForm.vue] --> B[composables/useAuth.ts]
        B -->|Fetch with Cookie| C[Nginx (77.67.1.106:80)]
    end

    subgraph "Reverse Proxy (Nginx)"
        C -->|location /api/auth/| D[Nuxt Nitro Node Service (127.0.0.1:3000)]
        C -->|location /api/| E[Go Control Service (127.0.0.1:18080)]
        C -->|location /| F[Static Cache / SSR]
    end

    subgraph "Nitro Server Layer (127.0.0.1:3000)"
        D --> G[server/middleware/auth.ts]
        G --> H[server/api/auth/*]
        H --> I[server/utils/auth.ts (Argon2id & SHA-256)]
        H --> J[server/db/index.ts (Drizzle ORM)]
    end

    subgraph "Database Layer (PostgreSQL 17)"
        J --> K[(attackbox_db)]
        J --> L[(attackbox_test)]
    end
```

**Tech Stack:**
- Nuxt 3.15+ (Nitro Engine)
- Vue 3.5
- Drizzle ORM / `postgres.js`
- `argon2` (Argon2id)
- PostgreSQL 17.11
- Nginx, systemd, Node.js 20

## Global Constraints

- 密码必须且仅使用 Argon2id 哈希。
- 会话凭证为 32 字节高熵随机串，Cookie 名称统一为 `attackbox_session`，参数 `httpOnly: true, secure: true, sameSite: 'lax', path: '/'`，数据库仅存储 SHA-256 摘要（32 字节）。
- 用户注册时强制赋予 `role: 'user'` 与 `status: 'pending'`，接口严禁接受客户端传入 role 或 status。
- 只有 `status: 'active'` 的用户被允许登录或读取受限资源。
- 密码重置时必须在事务内将该用户的所有现存有效会话标记为 `revoked_at = now()`。
- 所有的编译、打包、服务部署与端到端集成测试必须在远程服务器 `ssh laji@77.67.1.106` 上执行验证。

---

### Task 1: 服务端安全工具集与依赖配置

**Files:**
- Modify: `web/package.json`
- Create: `web/server/utils/auth.ts`
- Create: `web/server/utils/mailer.ts`
- Create: `web/tests/auth-crypto.test.js`

**Interfaces:**
- Consumes: `argon2`, `node:crypto`
- Produces: `hashPassword`, `verifyPassword`, `generateSecureToken`, `hashToken`, `createSessionCookie`, `clearSessionCookie`, `sendVerificationEmail`, `sendResetPasswordEmail`

- [ ] **Step 1: 安装 argon2 依赖并在 package.json 中配置**
在 `web/package.json` 的 `dependencies` 中添加 `argon2`。运行 `npm install argon2`。

- [ ] **Step 2: 编写 auth-crypto 单元测试 (TDD)**
在 `web/tests/auth-crypto.test.js` 中编写测试：
- 密码 Argon2id 哈希与正确比对。
- 错误密码比对返回 false。
- 32 字节高熵令牌生成与 SHA-256 摘要计算（确保输出 Buffer 长度为 32）。

- [ ] **Step 3: 实现 server/utils/auth.ts**
实现密码哈希、密码校验、高熵 Token 生成、SHA-256 摘要计算，以及会话 Cookie 设置与清理函数。

- [ ] **Step 4: 实现 server/utils/mailer.ts**
实现邮件抽象服务，当前环境输出激活/重置链接至日志，预留 SMTP 驱动接口。

- [ ] **Step 5: 运行本地单元测试验证通过**
运行 `node --test tests/auth-crypto.test.js`，确保测试通过。

---

### Task 2: 服务端用户会话中间件与管理员初始化 CLI

**Files:**
- Create: `web/server/middleware/auth.ts`
- Create: `web/server/scripts/init-admin.mjs`

**Interfaces:**
- Consumes: `event.context.user`, `db`, `authSessions`, `users`
- Produces: 自动解析会话的中间件上下文与受控创建 admin 的 CLI 命令

- [ ] **Step 1: 实现 server/middleware/auth.ts 会话解析中间件**
读取请求中的 `attackbox_session` Cookie；若存在，通过 SHA-256 摘要查询未撤销且未过期的 Session；校验关联用户 `status === 'active'`；若有效，将用户挂载至 `event.context.user` 并异步更新 `last_seen_at`。

- [ ] **Step 2: 实现 server/scripts/init-admin.mjs 管理员初始化脚本**
支持传入邮箱、密码、展示名；检查是否已存在管理员；若不存在，直接生成 Argon2id 密码并插入 `role: 'admin'`, `status: 'active'`, `email_verified_at: now()` 的受控管理员账号。并在 `package.json` 添加 `"admin:init"` 脚本。

---

### Task 3: 实现全套 Nitro 认证 API 路由

**Files:**
- Create: `web/server/api/auth/register.post.ts`
- Create: `web/server/api/auth/verify-email.post.ts`
- Create: `web/server/api/auth/login.post.ts`
- Create: `web/server/api/auth/me.get.ts`
- Create: `web/server/api/auth/logout.post.ts`
- Create: `web/server/api/auth/forgot-password.post.ts`
- Create: `web/server/api/auth/reset-password.post.ts`

**Interfaces:**
- Consumes: `H3Event`, `readBody`, `setResponseStatus`, `db`
- Produces: 符合 REST 规范与设计契约的 JSON 响应

- [ ] **Step 1: 实现 register.post.ts**
校验邮箱格式与密码强度，防重复注册；插入 `users` (pending) 与 `email_tokens` (verify_email)，调用 Mailer。

- [ ] **Step 2: 实现 verify-email.post.ts**
单事务核对有效未消费的 verify_email Token，将用户状态激活为 `active`，消费 Token。

- [ ] **Step 3: 实现 login.post.ts**
核对凭据，检查 `status === 'active'`，验证 Argon2id，生成 32 字节会话，存 SHA-256 摘要，写 HttpOnly Cookie。

- [ ] **Step 4: 实现 me.get.ts 与 logout.post.ts**
`me.get.ts` 返回当前 `event.context.user`；`logout.post.ts` 吊销 Session 并清除 Cookie。

- [ ] **Step 5: 实现 forgot-password.post.ts 与 reset-password.post.ts**
找回密码安全防枚举；重置密码校验 Token、更新密码并撤销所有已登录 Session。

---

### Task 4: 前端 Composable 与表单对接

**Files:**
- Create: `web/composables/useAuth.ts`
- Modify: `web/components/auth/LoginForm.vue`
- Modify: `web/components/auth/RegisterForm.vue`
- Modify: `web/components/auth/RecoverForm.vue`

**Interfaces:**
- Consumes: `/api/auth/*` 接口
- Produces: 前端真实登录、注册、重置响应式状态与表单提交绑定

- [ ] **Step 1: 实现 web/composables/useAuth.ts**
使用 Nuxt `useState` 维护全局 `currentUser`，封装 `login`, `register`, `logout`, `fetchMe`, `verifyEmail`, `forgotPassword`, `resetPassword` 方法，处理错误信息。

- [ ] **Step 2: 对接 LoginForm.vue**
将原有的 `setTimeout` 模拟逻辑替换为调用 `useAuth().login(...)`；处理真实网络错误（如密码错误、账号未激活）；登录成功后更新状态。

- [ ] **Step 3: 对接 RegisterForm.vue**
调用 `useAuth().register(...)`，成功后展示真实引导文案与激活指引。

- [ ] **Step 4: 对接 RecoverForm.vue**
调用 `useAuth().forgotPassword(...)`，提交真实重置请求。

- [ ] **Step 5: 验证本地现有 29 个前端测试全部继续通过**
运行 `npm test`，确保原有前台结构、文案与组件测试不产生任何退化。

---

### Task 5: 编写远程端到端自动化验证套件

**Files:**
- Create: `web/tests/auth-remote-e2e.mjs`

**Interfaces:**
- Consumes: 远程 HTTP API (`http://127.0.0.1:3000/api/auth/` 或 `http://77.67.1.106/api/auth/`)
- Produces: 自动化断言结果

- [ ] **Step 1: 编写完整的端到端认证测试脚本**
编写自动化场景测试用例：
1. 注册新用户 (`alice_test@example.com`)，验证数据库生成 pending 账号与 verify_email 令牌。
2. 尝试登录 pending 账号，验证返回 403 `email_not_verified`。
3. 消费 verify_email 令牌，验证账号更新为 `active`。
4. 再次登录，验证返回 200 并获取 `attackbox_session` HttpOnly Cookie。
5. 携带 Cookie 请求 `GET /api/auth/me`，验证身份正确。
6. 发起找回密码与重置密码，验证旧 Session 自动失效（携带旧 Cookie 请求 me 返回 401）。
7. 使用新密码重新登录，验证登录成功；执行登出，验证 Cookie 被清理。

---

### Task 6: 远程部署与 Nginx 反向代理配置

**Files:**
- Create: `scripts/deploy_auth_nitro.ps1`
- Remote: `/etc/systemd/system/attackbox-web.service`
- Remote: `/etc/nginx/sites-enabled/default`

**Interfaces:**
- Consumes: 本地编译产物、SSH 凭据
- Produces: 远程常驻 Node 3000 服务与 Nginx `/api/auth/` 代理

- [ ] **Step 1: 编写一键部署与远程验证脚本 deploy_auth_nitro.ps1**
1. 本地执行 `npm run build` 生成 Nitro 生产产物 `.output/`。
2. 打包上传至远程服务器 `/var/www/attackbox-web/`。
3. 配置 systemd 服务 `attackbox-web.service` 自动重启与常驻运行。
4. 更新 Nginx 配置：在 `location /api/` 前配置 `location /api/auth/ { proxy_pass http://127.0.0.1:3000; ... }` 并重载 Nginx。
5. 在远程运行 `tests/auth-remote-e2e.mjs`，捕获所有端到端用例执行状态。
6. 验证公网 HTTP 访问认证接口正常，出具完整测试报告。
