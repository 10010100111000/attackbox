// web/server/scripts/init-admin.mjs
// AttackBox Controlled Admin Initializer
import { hash, Algorithm } from '@node-rs/argon2';
import postgres from 'postgres';

const args = process.argv.slice(2);
const email = args[0]?.trim().toLowerCase();
const password = args[1];
const displayName = args[2]?.trim() || 'System Administrator';

if (!email || !password) {
  console.error('Usage: node server/scripts/init-admin.mjs <email> <password> [displayName]');
  process.exit(1);
}

const DATABASE_URL = process.env.DATABASE_URL || 'postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db';

async function main() {
  console.log(`[ADMIN-INIT] Initializing admin account: ${email}...`);
  const sql = postgres(DATABASE_URL, { max: 1 });

  try {
    const passwordHash = await hash(password, {
      algorithm: Algorithm.Argon2id,
      memoryCost: 65536,
      timeCost: 3,
      parallelism: 1,
    });
    const now = new Date();

    const [user] = await sql`
      INSERT INTO users (email, display_name, password_hash, role, status, email_verified_at, updated_at)
      VALUES (
        ${email},
        ${displayName},
        ${passwordHash},
        'admin',
        'active',
        ${now},
        ${now}
      )
      ON CONFLICT (email) DO UPDATE
      SET role = 'admin',
          status = 'active',
          password_hash = EXCLUDED.password_hash,
          email_verified_at = EXCLUDED.email_verified_at,
          updated_at = EXCLUDED.updated_at
      RETURNING id, email, role, status, display_name;
    `;

    console.log(`[ADMIN-INIT] Success! Admin account active:`, user);
  } finally {
    await sql.end();
  }
}

main().catch((err) => {
  console.error('[ADMIN-INIT] Error:', err);
  process.exit(1);
});
