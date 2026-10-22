// web/tests/admin-publish-remote-e2e.mjs
// AttackBox Phase 2 Step 3.3: Module Publish & Snapshot Consistency E2E Test Suite
import test from 'node:test';
import assert from 'node:assert/strict';

const BASE_URL = process.env.BASE_URL || 'http://127.0.0.1:3000';

console.log('--------------------------------------------------');
console.log('Starting AttackBox Admin Publish Remote E2E Tests');
console.log(`Base URL: ${BASE_URL}`);
console.log('--------------------------------------------------');

let adminCookie = '';
let testModuleId = '';
let testChapterId = '';
let firstRevisionId = '';

function extractSessionCookie(res) {
  const getSet = typeof res.headers.getSetCookie === 'function' ? res.headers.getSetCookie() : [];
  if (getSet.length > 0) {
    for (const c of getSet) {
      const match = c.match(/attackbox_session=[^;]+/);
      if (match) return match[0];
    }
  }
  const raw = res.headers.get('set-cookie') || '';
  const match = raw.match(/attackbox_session=[^;]+/);
  return match ? match[0] : '';
}

test('0. Setup: Admin Authentication & Empty Module Creation', async () => {
  // 管理员登录
  const loginRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@attackbox.org',
      password: 'AttackBoxAdmin2026!',
    }),
  });
  assert.equal(loginRes.status, 200);
  adminCookie = extractSessionCookie(loginRes);
  assert.ok(adminCookie);

  // 创建一个全新的测试模块（初始没有章节）
  const slug = `publish-flow-${Date.now()}`;
  const createModRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug,
      title: '发布一致性测试模块',
      summary: '测试从草稿校验、封存到发布的完整原子事务',
    }),
  });
  const createModData = await createModRes.json();
  assert.equal(createModRes.status, 200);
  testModuleId = createModData.data.module.id;
  firstRevisionId = createModData.data.draft.id;
  assert.ok(testModuleId);
  assert.ok(firstRevisionId);

  console.log(`[PASS] Setup complete. Created test module: ${slug} (${testModuleId})`);
});

test('1. Publishing Empty Module (0 Chapters) Must Be Blocked with 400', async () => {
  const pubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/publish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  const data = await pubRes.json();
  assert.equal(pubRes.status, 400);
  assert.match(data.message || data.statusMessage || '', /章节|无法发布/);

  console.log('[PASS] Empty module publish properly rejected with 400');
});

test('2. Publishing with Missing Quiz Answers Must Be Blocked with 400', async () => {
  // 2.1 添加一个章节
  const chRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/chapters`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug: 'quiz-integrity-ch1',
      title: '第一章 权限提升测试',
      access: 'public',
    }),
  });
  const chData = await chRes.json();
  assert.equal(chRes.status, 200);
  testChapterId = chData.data.chapter.id;
  assert.ok(testChapterId);

  // 2.2 为该章节添加一道题目，故意留空 answer（草稿阶段允许，但发布阶段不允许）
  const saveChRes = await fetch(`${BASE_URL}/api/admin/chapters/${testChapterId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      moduleId: testModuleId,
      title: '第一章 权限提升测试',
      bodyMarkdown: '# Linux 提权\n\n通过 sudo -l 查看配置...',
      questions: [
        {
          position: 1,
          promptMarkdown: '修改用户密码的 Linux 命令是什么？',
          normalization: 'trim',
          // 故意不传 answer
        },
      ],
    }),
  });
  assert.equal(saveChRes.status, 200);

  // 2.3 尝试发布，必须被题目缺失答案拦截
  const pubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/publish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  const pubData = await pubRes.json();
  assert.equal(pubRes.status, 400);
  assert.match(pubData.message || pubData.statusMessage || '', /标准答案|答案/);

  console.log('[PASS] Missing quiz answer blocks publish with 400');
});

test('3. Complete & Valid Draft Publishes Atomically with Sealed Snapshots', async () => {
  // 3.1 完善答案
  const saveChRes = await fetch(`${BASE_URL}/api/admin/chapters/${testChapterId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      moduleId: testModuleId,
      title: '第一章 权限提升测试 (完整版)',
      bodyMarkdown: '# Linux 提权\n\n通过 sudo -l 查看配置...',
      questions: [
        {
          position: 1,
          promptMarkdown: '修改用户密码的 Linux 命令是什么？',
          normalization: 'trim',
          answer: 'passwd', // 设置合法答案
        },
      ],
    }),
  });
  assert.equal(saveChRes.status, 200);

  // 3.2 执行发布
  const pubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/publish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  const pubData = await pubRes.json();
  assert.equal(pubRes.status, 200);
  assert.equal(pubData.data.module.publishedRevisionId, firstRevisionId);
  assert.equal(pubData.data.publishedRevision.state, 'published');
  assert.ok(pubData.data.publishedRevision.publishedAt);

  // 3.3 查询模块详情验证已发布状态与封存状态
  const detailRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}`, {
    headers: { Cookie: adminCookie },
  });
  const detailData = await detailRes.json();
  assert.equal(detailRes.status, 200);
  assert.equal(detailData.data.published.id, firstRevisionId);
  assert.equal(detailData.data.published.state, 'published');
  assert.equal(detailData.data.draft, null); // 发布后原草稿变为 published，当前无草稿
  assert.equal(detailData.data.chapters.length, 1);
  assert.equal(detailData.data.chapters[0].isSealed, true); // 验证章节快照已被封存！

  console.log('[PASS] Module published atomically: published_revision_id set, state=published, snapshot sealed');
});

test('4. Modifying Published Module Automatically Forks Next Draft (Rev 2)', async () => {
  // 4.1 在已发布模块上直接调用 update draft，系统应自动派生 Revision 2 草稿
  const updateRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      title: '发布一致性测试模块 (第二版草稿)',
      summary: '升级说明与新章节规划',
    }),
  });
  const updateData = await updateRes.json();
  assert.equal(updateRes.status, 200);
  assert.equal(updateData.data.revisionNo, 2);
  assert.equal(updateData.data.state, 'draft');

  // 4.2 验证此时同时存在：已发布的 Revision 1 与新草稿 Revision 2
  const detailRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}`, {
    headers: { Cookie: adminCookie },
  });
  const detailData = await detailRes.json();
  assert.equal(detailRes.status, 200);
  assert.equal(detailData.data.published.revisionNo, 1);
  assert.equal(detailData.data.draft.revisionNo, 2);
  // 新草稿自动继承了上一版本的章节编排
  assert.equal(detailData.data.chapters.length, 1);

  console.log('[PASS] Published module successfully forks revision 2 draft while keeping revision 1 live');
});

test('5. Unpublish Module Resets Pointer while Preserving Revisions', async () => {
  // 5.1 执行下架
  const unpubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/unpublish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  const unpubData = await unpubRes.json();
  assert.equal(unpubRes.status, 200);
  assert.equal(unpubData.data.publishedRevisionId, null);

  // 5.2 再次获取详情
  const detailRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}`, {
    headers: { Cookie: adminCookie },
  });
  const detailData = await detailRes.json();
  assert.equal(detailRes.status, 200);
  assert.equal(detailData.data.module.publishedRevisionId, null);
  assert.equal(detailData.data.published, null);

  // 5.3 重复下架返回 400
  const dupUnpubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/unpublish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  assert.equal(dupUnpubRes.status, 400);

  console.log('[PASS] Module unpublish resets pointer and handles idempotency');
});

test.after(async () => {
  console.log('--------------------------------------------------');
  console.log('AttackBox Admin Publish Remote E2E Tests Complete!');
  console.log('--------------------------------------------------');
});
