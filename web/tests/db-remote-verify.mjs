// web/tests/db-remote-verify.mjs
// AttackBox Phase 2 Database & Auth Schema Remote Verification Suite
import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import postgres from 'postgres';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const DATABASE_URL = process.env.DATABASE_URL || 'postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_test';

console.log('--------------------------------------------------');
console.log('Starting AttackBox Database Remote Verification Suite');
console.log(`Connecting to: ${DATABASE_URL.replace(/:[^:@]+@/, ':****@')}`);
console.log('--------------------------------------------------');

const sql = postgres(DATABASE_URL, {
  max: 1,
  idle_timeout: 10,
  connect_timeout: 5,
});

test('0. Database Connectivity', async () => {
  const [res] = await sql`SELECT current_database(), current_user, version()`;
  assert.equal(res.current_database, 'attackbox_test');
  assert.equal(res.current_user, 'attackbox');
  console.log(`[PASS] Connected as ${res.current_user} to ${res.current_database}`);
});

test('1. Migration Execution & Schema Integrity', async () => {
  // 清理现有测试表以确保幂等干净环境
  await sql`DROP TABLE IF EXISTS audit_logs CASCADE`;
  await sql`DROP TABLE IF EXISTS auth_sessions CASCADE`;
  await sql`DROP TABLE IF EXISTS email_tokens CASCADE`;
  await sql`DROP TABLE IF EXISTS users CASCADE`;

  // 寻找并执行 migrations 中的 sql
  const migrationsDir = path.resolve(__dirname, '../server/db/migrations');
  const files = fs.readdirSync(migrationsDir).filter(f => f.endsWith('.sql')).sort();
  assert.ok(files.length > 0, 'No migration files found');

  for (const file of files) {
    const filePath = path.join(migrationsDir, file);
    const content = fs.readFileSync(filePath, 'utf-8');
    const statements = content
      .split('--> statement-breakpoint')
      .map(s => s.trim())
      .filter(Boolean);

    for (const stmt of statements) {
      await sql.unsafe(stmt);
    }
  }

  // 校验 4 张核心表已建立
  const tables = await sql`
    SELECT table_name 
    FROM information_schema.tables 
    WHERE table_schema = 'public' 
      AND table_name IN ('users', 'auth_sessions', 'email_tokens', 'audit_logs')
  `;
  const tableNames = tables.map(t => t.table_name).sort();
  assert.deepEqual(tableNames, ['audit_logs', 'auth_sessions', 'email_tokens', 'users']);
  console.log(`[PASS] Tables created: ${tableNames.join(', ')}`);
});

test('2. Users Table Schema & Constraints Validation', async () => {
  // 2.1 正常插入测试用户
  const [user1] = await sql`
    INSERT INTO users (email, display_name, password_hash, role, status)
    VALUES ('alice@example.com', 'Alice In Cyber', 'argon2id$mockhash', 'user', 'pending')
    RETURNING *
  `;
  assert.ok(user1.id);
  assert.equal(user1.email, 'alice@example.com');
  assert.equal(user1.display_name, 'Alice In Cyber');
  assert.equal(user1.role, 'user');
  assert.equal(user1.status, 'pending');

  // 2.2 验证大写邮箱被 CHECK 拒绝 (users_email_lower_trimmed_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name)
        VALUES ('Bob@EXAMPLE.com', 'Bob')
      `;
    },
    /check constraint.*users_email_lower_trimmed_check/i,
    'Uppercase email should violate check constraint'
  );

  // 2.3 验证首尾空格邮箱被 CHECK 拒绝
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name)
        VALUES ('  charlie@example.com ', 'Charlie')
      `;
    },
    /check constraint.*users_email_lower_trimmed_check/i,
    'Untrimmed email should violate check constraint'
  );

  // 2.4 验证邮箱唯一索引约束 (users_email_unique_idx)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name)
        VALUES ('alice@example.com', 'Another Alice')
      `;
    },
    /unique constraint.*users_email_unique_idx/i,
    'Duplicate email should violate unique constraint'
  );

  // 2.5 验证非法 role 枚举被 CHECK 拒绝 (users_role_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name, role)
        VALUES ('david@example.com', 'David', 'superadmin')
      `;
    },
    /check constraint.*users_role_check/i,
    'Invalid role should violate check constraint'
  );

  // 2.6 验证非法 status 枚举被 CHECK 拒绝 (users_status_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name, status)
        VALUES ('eve@example.com', 'Eve', 'deleted')
      `;
    },
    /check constraint.*users_status_check/i,
    'Invalid status should violate check constraint'
  );

  // 2.7 验证空白 display_name 被 CHECK 拒绝 (users_display_name_non_empty_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO users (email, display_name)
        VALUES ('frank@example.com', '   ')
      `;
    },
    /check constraint.*users_display_name_non_empty_check/i,
    'Empty display_name should violate check constraint'
  );

  console.log('[PASS] Users table constraints & checks verified');
});

test('3. Auth Sessions Constraints & Token Hash Validation', async () => {
  const [user] = await sql`SELECT id FROM users WHERE email = 'alice@example.com'`;
  const validHash = crypto.randomBytes(32);
  const invalidHashShort = crypto.randomBytes(16);

  // 3.1 正常插入会话 (32 字节 bytea, expires_at > created_at)
  const [session] = await sql`
    INSERT INTO auth_sessions (user_id, token_hash, expires_at, ip, user_agent)
    VALUES (
      ${user.id},
      ${validHash},
      now() + interval '24 hours',
      '192.168.1.100'::inet,
      'Mozilla/5.0 Test Suite'
    )
    RETURNING *
  `;
  assert.ok(session.id);
  assert.equal(session.user_id, user.id);

  // 3.2 验证非 32 字节 token_hash 被 CHECK 拒绝 (auth_sessions_token_hash_len_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO auth_sessions (user_id, token_hash, expires_at)
        VALUES (${user.id}, ${invalidHashShort}, now() + interval '24 hours')
      `;
    },
    /check constraint.*auth_sessions_token_hash_len_check/i,
    'Short token hash should violate 32-byte check'
  );

  // 3.3 验证 expires_at <= created_at 被 CHECK 拒绝
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO auth_sessions (user_id, token_hash, created_at, expires_at)
        VALUES (${user.id}, ${crypto.randomBytes(32)}, now(), now() - interval '1 hour')
      `;
    },
    /check constraint.*auth_sessions_expires_after_created_check/i,
    'Past expires_at should violate check'
  );

  // 3.4 验证 token_hash 唯一索引
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO auth_sessions (user_id, token_hash, expires_at)
        VALUES (${user.id}, ${validHash}, now() + interval '24 hours')
      `;
    },
    /unique constraint.*auth_sessions_token_hash_unique_idx/i,
    'Duplicate token_hash should violate unique index'
  );

  console.log('[PASS] Auth sessions constraints verified');
});

test('4. Email Tokens Constraints Validation', async () => {
  const [user] = await sql`SELECT id FROM users WHERE email = 'alice@example.com'`;
  const validHash = crypto.randomBytes(32);

  // 4.1 正常插入 email_token
  const [token] = await sql`
    INSERT INTO email_tokens (user_id, target_email, purpose, token_hash, expires_at)
    VALUES (
      ${user.id},
      'alice@example.com',
      'verify_email',
      ${validHash},
      now() + interval '24 hours'
    )
    RETURNING *
  `;
  assert.ok(token.id);

  // 4.2 验证非法 purpose 枚举被 CHECK 拒绝 (email_tokens_purpose_check)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO email_tokens (user_id, target_email, purpose, token_hash, expires_at)
        VALUES (
          ${user.id},
          'alice@example.com',
          'invalid_action',
          ${crypto.randomBytes(32)},
          now() + interval '24 hours'
        )
      `;
    },
    /check constraint.*email_tokens_purpose_check/i,
    'Invalid purpose should violate check'
  );

  console.log('[PASS] Email tokens constraints verified');
});

test('5. Foreign Key Cascade Deletion (Users -> Sessions & Tokens)', async () => {
  const [user] = await sql`SELECT id FROM users WHERE email = 'alice@example.com'`;
  
  // 确认删除前有 session 和 token
  const sessionsBefore = await sql`SELECT count(*) FROM auth_sessions WHERE user_id = ${user.id}`;
  const tokensBefore = await sql`SELECT count(*) FROM email_tokens WHERE user_id = ${user.id}`;
  assert.ok(Number(sessionsBefore[0].count) > 0);
  assert.ok(Number(tokensBefore[0].count) > 0);

  // 删除用户
  await sql`DELETE FROM users WHERE id = ${user.id}`;

  // 验证 session 与 token 均被自动 CASCADE 删除
  const sessionsAfter = await sql`SELECT count(*) FROM auth_sessions WHERE user_id = ${user.id}`;
  const tokensAfter = await sql`SELECT count(*) FROM email_tokens WHERE user_id = ${user.id}`;
  assert.equal(Number(sessionsAfter[0].count), 0);
  assert.equal(Number(tokensAfter[0].count), 0);

  console.log('[PASS] Foreign key CASCADE verified for sessions and tokens');
});

test('6. Audit Logs RESTRICT Protection on User Deletion', async () => {
  // 6.1 插入新用户及关联 audit_log
  const [actor] = await sql`
    INSERT INTO users (email, display_name, role, status)
    VALUES ('admin@example.com', 'Security Auditor', 'admin', 'active')
    RETURNING id
  `;

  const [log] = await sql`
    INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata, ip)
    VALUES (
      ${actor.id},
      'system.init',
      'system',
      gen_random_uuid(),
      '{"reason": "phase2 test initialization"}'::jsonb,
      '127.0.0.1'::inet
    )
    RETURNING *
  `;
  assert.ok(log.id);
  assert.equal(log.actor_user_id, actor.id);

  // 6.2 尝试删除该用户，断言因 ON DELETE RESTRICT 被拒绝
  await assert.rejects(
    async () => {
      await sql`DELETE FROM users WHERE id = ${actor.id}`;
    },
    /foreign key constraint.*audit_logs_actor_user_id_users_id_fk/i,
    'Deleting user with audit logs must fail due to ON DELETE RESTRICT'
  );

  // 6.3 清理审计日志后再删除用户，应当成功
  await sql`DELETE FROM audit_logs WHERE id = ${log.id}`;
  await sql`DELETE FROM users WHERE id = ${actor.id}`;

  console.log('[PASS] Audit logs ON DELETE RESTRICT verified');
});

test('Teardown & Cleanup', async () => {
  await sql.end();
  console.log('--------------------------------------------------');
  console.log('ALL REMOTE DATABASE TESTS COMPLETED SUCCESSFULLY!');
  console.log('--------------------------------------------------');
});
