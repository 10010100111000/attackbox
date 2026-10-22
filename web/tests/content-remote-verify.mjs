// web/tests/content-remote-verify.mjs
// AttackBox Phase 2 Step 3.1: Content Management Schema & Constraints Remote Verification Suite
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
console.log('Starting AttackBox Content Schema Remote Verification Suite');
console.log(`Connecting to: ${DATABASE_URL.replace(/:[^:@]+@/, ':****@')}`);
console.log('--------------------------------------------------');

const sql = postgres(DATABASE_URL, {
  max: 1,
  idle_timeout: 10,
  connect_timeout: 5,
});

let testUserId;

test('0. Database Connectivity', async () => {
  const [res] = await sql`SELECT current_database(), current_user, version()`;
  assert.equal(res.current_database, 'attackbox_test');
  assert.equal(res.current_user, 'attackbox');
  console.log(`[PASS] Connected as ${res.current_user} to ${res.current_database}`);
});

test('1. Migration Execution & Full 12 Tables Check', async () => {
  // 先清理旧表保证测试幂等干净
  const tablesToDrop = [
    'chapter_revision_assets',
    'chapter_questions',
    'module_revision_chapters',
    'chapter_revisions',
    'chapters',
    'module_revisions',
    'modules',
    'assets',
    'audit_logs',
    'auth_sessions',
    'email_tokens',
    'users',
  ];
  for (const tbl of tablesToDrop) {
    await sql.unsafe(`DROP TABLE IF EXISTS "${tbl}" CASCADE`);
  }

  // 执行所有迁移文件
  const migrationsDir = path.resolve(__dirname, '../server/db/migrations');
  const files = fs.readdirSync(migrationsDir).filter(f => f.endsWith('.sql')).sort();
  assert.ok(files.length >= 2, 'Expected at least 2 migration files');

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

  const existingTables = await sql`
    SELECT table_name 
    FROM information_schema.tables 
    WHERE table_schema = 'public'
    ORDER BY table_name ASC
  `;
  const names = existingTables.map(t => t.table_name);
  for (const tbl of tablesToDrop) {
    assert.ok(names.includes(tbl), `Table ${tbl} should exist`);
  }
  console.log(`[PASS] All 12 tables created successfully: ${tablesToDrop.join(', ')}`);

  // 创建一个测试管理员用户用于后续外键引用
  const [user] = await sql`
    INSERT INTO users (email, display_name, password_hash, role, status)
    VALUES ('content-admin@attackbox.org', 'Content Admin', 'argon2id$mock', 'admin', 'active')
    RETURNING id
  `;
  testUserId = user.id;
  assert.ok(testUserId);
});

test('2. Assets Table Schema & Constraints Validation', async () => {
  const dummySha256 = crypto.createHash('sha256').update('image-content-v1').digest();
  assert.equal(dummySha256.length, 32);

  // 2.1 正常插入资产
  const [asset1] = await sql`
    INSERT INTO assets (storage_key, original_name, media_type, byte_size, sha256, visibility, uploaded_by)
    VALUES ('covers/linux-intro.png', 'linux-intro.png', 'image/png', 204800, ${dummySha256}, 'public', ${testUserId})
    RETURNING *
  `;
  assert.ok(asset1.id);
  assert.equal(asset1.storage_key, 'covers/linux-intro.png');
  assert.equal(asset1.visibility, 'public');

  // 2.2 重复 storage_key 必须触发唯一约束冲突
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO assets (storage_key, original_name, media_type, byte_size, sha256, visibility, uploaded_by)
        VALUES ('covers/linux-intro.png', 'duplicate.png', 'image/png', 1024, ${dummySha256}, 'public', ${testUserId})
      `;
    },
    /duplicate key.*assets_storage_key_unique_idx|unique constraint/i,
    'Duplicate storage_key must be rejected'
  );

  // 2.3 非法 visibility 触发 check 约束
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO assets (storage_key, original_name, media_type, byte_size, sha256, visibility, uploaded_by)
        VALUES ('covers/invalid.png', 'invalid.png', 'image/png', 1024, ${dummySha256}, 'hidden', ${testUserId})
      `;
    },
    /check constraint.*assets_visibility_check/i,
    'Invalid visibility must violate check constraint'
  );

  // 2.4 非法 sha256 长度 (例如 16 字节) 触发 check 约束
  const shortSha = Buffer.alloc(16);
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO assets (storage_key, original_name, media_type, byte_size, sha256, visibility, uploaded_by)
        VALUES ('covers/short-hash.png', 'short.png', 'image/png', 1024, ${shortSha}, 'private', ${testUserId})
      `;
    },
    /check constraint.*assets_sha256_len_check/i,
    '16-byte sha256 must violate check constraint'
  );

  console.log('[PASS] Assets constraints verified (storage_key unique, visibility check, sha256 32-byte check)');
});

test('3. Modules & Module Revisions (Single Draft Partial Unique Index)', async () => {
  // 3.1 模块 slug 校验
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO modules (slug, created_by)
        VALUES ('Invalid Slug!', ${testUserId})
      `;
    },
    /check constraint.*modules_slug_format_check/i,
    'Slug with uppercase and special chars must be rejected'
  );

  const [moduleA] = await sql`
    INSERT INTO modules (slug, created_by)
    VALUES ('linux-security-basics', ${testUserId})
    RETURNING *
  `;
  assert.ok(moduleA.id);

  // 3.2 插入第 1 个修订版本 (draft)
  const [rev1] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, summary, difficulty, created_by)
    VALUES (${moduleA.id}, 1, 'draft', 'Linux 安全基础', '入门Linux防御技术', 'beginner', ${testUserId})
    RETURNING *
  `;
  assert.ok(rev1.id);
  assert.equal(rev1.state, 'draft');

  // 3.3 尝试向同一模块插入第 2 个 draft，必须被部分唯一索引拒绝！
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
        VALUES (${moduleA.id}, 2, 'draft', 'Linux 安全基础-第二个草稿', ${testUserId})
      `;
    },
    /duplicate key.*module_revisions_single_draft_idx|unique constraint/i,
    'Single draft per module must be strictly enforced by partial index'
  );

  // 3.4 将 rev1 状态转为 published
  await sql`
    UPDATE module_revisions 
    SET state = 'published', published_at = NOW() 
    WHERE id = ${rev1.id}
  `;

  // 3.5 发布后，新建第 2 个版本 draft 必须顺利成功！
  const [rev2] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
    VALUES (${moduleA.id}, 2, 'draft', 'Linux 安全基础 v2 草稿', ${testUserId})
    RETURNING *
  `;
  assert.ok(rev2.id);
  assert.equal(rev2.revision_no, 2);
  assert.equal(rev2.state, 'draft');

  console.log('[PASS] Module revisions single draft partial index verified');
});

test('4. Cross-Module Isolation via Composite Foreign Keys', async () => {
  // 创建模块 A 与 模块 B
  const [modA] = await sql`INSERT INTO modules (slug, created_by) VALUES ('module-alpha', ${testUserId}) RETURNING *`;
  const [modB] = await sql`INSERT INTO modules (slug, created_by) VALUES ('module-beta', ${testUserId}) RETURNING *`;

  // 模块 A 版本 A1
  const [revA1] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
    VALUES (${modA.id}, 1, 'published', 'Alpha Rev 1', ${testUserId})
    RETURNING *
  `;

  // 模块 B 版本 B1
  const [revB1] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
    VALUES (${modB.id}, 1, 'published', 'Beta Rev 1', ${testUserId})
    RETURNING *
  `;

  // 模块 A 章节 A-Ch1
  const [chA1] = await sql`
    INSERT INTO chapters (module_id, slug)
    VALUES (${modA.id}, 'alpha-intro')
    RETURNING *
  `;
  const [chA1Rev1] = await sql`
    INSERT INTO chapter_revisions (chapter_id, title, access, body_markdown, is_sealed, created_by)
    VALUES (${chA1.id}, 'Alpha Intro Title', 'public', '# Alpha Content', true, ${testUserId})
    RETURNING *
  `;

  // 模块 B 章节 B-Ch1
  const [chB1] = await sql`
    INSERT INTO chapters (module_id, slug)
    VALUES (${modB.id}, 'beta-intro')
    RETURNING *
  `;
  const [chB1Rev1] = await sql`
    INSERT INTO chapter_revisions (chapter_id, title, access, body_markdown, is_sealed, created_by)
    VALUES (${chB1.id}, 'Beta Intro Title', 'public', '# Beta Content', true, ${testUserId})
    RETURNING *
  `;

  // 4.1 攻击性测试：尝试将 模块B的章节 B-Ch1 插入 模块A的版本 A1 清单中
  // 由于复合外键 mrc_chapter_fk (chapter_id, module_id) REFERENCES chapters(id, module_id)
  // 当 module_revision_chapters.module_id 传 modA.id 时，(chB1.id, modA.id) 在 chapters 中不存在！
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO module_revision_chapters (module_revision_id, module_id, chapter_id, chapter_revision_id, position)
        VALUES (${revA1.id}, ${modA.id}, ${chB1.id}, ${chB1Rev1.id}, 1)
      `;
    },
    /foreign key constraint.*mrc_chapter_fk/i,
    'Cross-module chapter assignment must be blocked by composite foreign key mrc_chapter_fk'
  );

  // 4.2 攻击性测试：尝试把 章节A-Ch1 的 revision 指定为 章节B-Ch1的 revision
  // 由于复合外键 mrc_chapter_revision_fk (chapter_revision_id, chapter_id) REFERENCES chapter_revisions(id, chapter_id)
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO module_revision_chapters (module_revision_id, module_id, chapter_id, chapter_revision_id, position)
        VALUES (${revA1.id}, ${modA.id}, ${chA1.id}, ${chB1Rev1.id}, 1)
      `;
    },
    /foreign key constraint.*mrc_chapter_revision_fk/i,
    'Mismatched chapter_revision must be blocked by composite foreign key mrc_chapter_revision_fk'
  );

  // 4.3 合法插入：模块A + 模块A章节 + 模块A章节版本
  const [validMRC] = await sql`
    INSERT INTO module_revision_chapters (module_revision_id, module_id, chapter_id, chapter_revision_id, position)
    VALUES (${revA1.id}, ${modA.id}, ${chA1.id}, ${chA1Rev1.id}, 1)
    RETURNING *
  `;
  assert.ok(validMRC);
  assert.equal(validMRC.position, 1);

  console.log('[PASS] Triple composite foreign key strictly prevents cross-module and mismatched revisions');
});

test('5. Published Revision Pointer Composite FK Enforcement', async () => {
  const [mod1] = await sql`INSERT INTO modules (slug, created_by) VALUES ('pointer-test-mod-1', ${testUserId}) RETURNING *`;
  const [mod2] = await sql`INSERT INTO modules (slug, created_by) VALUES ('pointer-test-mod-2', ${testUserId}) RETURNING *`;

  const [rev2] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
    VALUES (${mod2.id}, 1, 'published', 'Mod2 Rev1', ${testUserId})
    RETURNING *
  `;

  // 5.1 尝试将 mod1 的 published_revision_id 指向 mod2 的 rev2
  // 必须被 modules_published_rev_fk 拒绝！
  await assert.rejects(
    async () => {
      await sql`
        UPDATE modules
        SET published_revision_id = ${rev2.id}
        WHERE id = ${mod1.id}
      `;
    },
    /foreign key constraint.*modules_published_rev_fk/i,
    'Published revision pointing to another module must be rejected by composite foreign key'
  );

  // 5.2 为 mod1 创建属于自身的 revision
  const [rev1] = await sql`
    INSERT INTO module_revisions (module_id, revision_no, state, title, created_by)
    VALUES (${mod1.id}, 1, 'published', 'Mod1 Rev1', ${testUserId})
    RETURNING *
  `;

  // 5.3 正确指向自身 revision，成功
  await sql`
    UPDATE modules
    SET published_revision_id = ${rev1.id}
    WHERE id = ${mod1.id}
  `;
  const [updatedMod1] = await sql`SELECT published_revision_id FROM modules WHERE id = ${mod1.id}`;
  assert.equal(updatedMod1.published_revision_id, rev1.id);

  console.log('[PASS] modules.published_revision_id composite foreign key verified');
});

test('6. Chapter Questions Constraints (HMAC & Positions)', async () => {
  // 获取一个已有的 chapter_revision
  const [chRev] = await sql`SELECT id FROM chapter_revisions LIMIT 1`;
  assert.ok(chRev);

  // 6.1 插入草稿题目 (answer_hmac 为 NULL)
  const [q1] = await sql`
    INSERT INTO chapter_questions (chapter_revision_id, position, prompt_markdown, normalization)
    VALUES (${chRev.id}, 1, '请问Linux查看当前用户的命令是什么？', 'trim')
    RETURNING *
  `;
  assert.ok(q1.id);
  assert.equal(q1.answer_hmac, null);

  // 6.2 插入带有 32 字节 HMAC 答案的题目
  const validHmac = crypto.createHmac('sha256', 'mock-secret').update('whoami').digest();
  assert.equal(validHmac.length, 32);

  const [q2] = await sql`
    INSERT INTO chapter_questions (chapter_revision_id, position, prompt_markdown, normalization, answer_hmac, answer_key_version)
    VALUES (${chRev.id}, 2, '请问查看内核版本的参数是 uname 后的哪个参数？', 'trim_casefold', ${validHmac}, 1)
    RETURNING *
  `;
  assert.ok(q2.id);
  assert.equal(q2.position, 2);

  // 6.3 插入非法长度的 answer_hmac (例如 20 字节)，必须被拒绝
  const invalidHmac = Buffer.alloc(20);
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO chapter_questions (chapter_revision_id, position, prompt_markdown, answer_hmac)
        VALUES (${chRev.id}, 3, '非法题目', ${invalidHmac})
      `;
    },
    /check constraint.*chapter_questions_answer_hmac_len_check/i,
    'Non-32-byte answer_hmac must be rejected'
  );

  // 6.4 重复 position 必须被拒绝
  await assert.rejects(
    async () => {
      await sql`
        INSERT INTO chapter_questions (chapter_revision_id, position, prompt_markdown)
        VALUES (${chRev.id}, 1, '重复序号题目')
      `;
    },
    /duplicate key.*chapter_questions_rev_pos_unique_idx|unique constraint/i,
    'Duplicate position within same chapter revision must be rejected'
  );

  console.log('[PASS] chapter_questions constraints (NULL or 32-byte HMAC, position uniqueness) verified');
});

test('7. Chapter Revision Assets & Restricted Deletion', async () => {
  const [chRev] = await sql`SELECT id FROM chapter_revisions LIMIT 1`;
  const [asset] = await sql`SELECT id FROM assets LIMIT 1`;
  assert.ok(chRev);
  assert.ok(asset);

  // 关联资产
  await sql`
    INSERT INTO chapter_revision_assets (chapter_revision_id, asset_id)
    VALUES (${chRev.id}, ${asset.id})
  `;

  // 尝试直接物理删除该被引用的资产，必须被 ON DELETE RESTRICT 拦截！
  await assert.rejects(
    async () => {
      await sql`DELETE FROM assets WHERE id = ${asset.id}`;
    },
    /foreign key constraint.*chapter_revision_assets_asset_id_assets_id_fk/i,
    'Deleting an asset referenced by a chapter revision must be restricted'
  );

  console.log('[PASS] Chapter revision assets association and RESTRICT protection verified');
});

test.after(async () => {
  await sql.end({ timeout: 2 });
  console.log('--------------------------------------------------');
  console.log('AttackBox Content Schema Verification Complete!');
  console.log('--------------------------------------------------');
});
