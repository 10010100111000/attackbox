#!/usr/bin/env bash
set -e
cd /var/www/attackbox-web/.output/server
node << 'EOF'
import('@node-rs/argon2').then(async ({ hash, Algorithm }) => {
  const { default: postgres } = await import('postgres');
  const sql = postgres('postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db');
  const passwordHash = await hash('AttackBoxAdmin2026!', {
    algorithm: Algorithm.Argon2id,
    memoryCost: 65536,
    timeCost: 3,
    parallelism: 1
  });
  const [user] = await sql`
    INSERT INTO users (email, display_name, password_hash, role, status, email_verified_at, updated_at)
    VALUES ('admin@attackbox.org', 'Super Admin', ${passwordHash}, 'admin', 'active', now(), now())
    ON CONFLICT (email) DO UPDATE
    SET role = 'admin', status = 'active', password_hash = EXCLUDED.password_hash, updated_at = now()
    RETURNING id, email, role, status, display_name;
  `;
  console.log('[SUCCESS] Admin account created:', user);
  await sql.end();
});
EOF
