// Called by scripts/manage.py admin. Password is supplied over stdin, never argv.
import { hash, Algorithm } from '@node-rs/argon2';
import postgres from 'postgres';

async function main() {
  if (process.argv.length !== 2 || !process.env.DATABASE_URL || process.stdin.isTTY) {
    throw new Error('Use sudo scripts/manage.py admin');
  }
  let input = '';
  for await (const chunk of process.stdin) {
    input += chunk;
    if (input.length > 16384) throw new Error('Invalid input');
  }
  const {email, password, displayName} = JSON.parse(input);
  if (typeof email !== 'string' || email.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) ||
      typeof password !== 'string' || password.length < 12 || password.length > 4096 ||
      typeof displayName !== 'string' || !displayName.trim() || displayName.length > 100) {
    throw new Error('Invalid input');
  }
  const passwordHash = await hash(password, {algorithm: Algorithm.Argon2id, memoryCost: 65536, timeCost: 3, parallelism: 1});
  const sql = postgres(process.env.DATABASE_URL, {max: 1});
  try {
    await sql.begin(async tx => {
      // Serialize simultaneous bootstrap commands. Never reset an existing account.
      await tx`SELECT pg_advisory_xact_lock(781452603)`;
      const admins = await tx`SELECT id FROM users WHERE role = 'admin' LIMIT 1`;
      if (admins.length) throw new Error('An administrator already exists');
      const existing = await tx`SELECT id FROM users WHERE email = ${email.toLowerCase()} LIMIT 1`;
      if (existing.length) throw new Error('Email already in use');
      await tx`INSERT INTO users (email, display_name, password_hash, role, status, email_verified_at, updated_at)
        VALUES (${email.toLowerCase()}, ${displayName.trim()}, ${passwordHash}, 'admin', 'active', now(), now())`;
    });
    console.log('管理员已创建。此命令不会重置已有账号。');
  } finally {
    await sql.end();
  }
}
main().catch(() => {
  console.error('管理员初始化失败：检查输入、数据库连接和迁移状态；已有管理员或邮箱时拒绝创建。');
  process.exitCode = 1;
});
