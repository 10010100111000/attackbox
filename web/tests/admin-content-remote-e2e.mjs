// web/tests/admin-content-remote-e2e.mjs
// AttackBox Phase 2 Step 3.2: Admin Content & Quiz API Remote E2E Test Suite
import test from 'node:test';
import assert from 'node:assert/strict';

const BASE_URL = process.env.BASE_URL || 'http://127.0.0.1:3000';

console.log('--------------------------------------------------');
console.log('Starting AttackBox Admin Content Remote E2E Tests');
console.log(`Base URL: ${BASE_URL}`);
console.log('--------------------------------------------------');

let adminCookie = '';
let regularUserCookie = '';
let createdModuleId = '';
let createdChapterIds = [];
let draftLockVersion = 1;

// 辅助：从 Set-Cookie 提取 session cookie
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

test('0. Setup: Regular User & Admin Authentication', async () => {
  // 0.1 注册并激活一个普通用户
  const regularEmail = `regular_${Date.now()}@example.com`;
  const regRes = await fetch(`${BASE_URL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: regularEmail,
      displayName: 'Regular User',
      password: 'RegularPassword2026!',
    }),
  });
  const regData = await regRes.json();
  assert.equal(regRes.status, 200, `Register failed: ${JSON.stringify(regData)}`);
  assert.ok(regData.verifyToken, `No verifyToken: ${JSON.stringify(regData)}`);

  // 激活
  const verifyRes = await fetch(`${BASE_URL}/api/auth/verify-email`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: regData.verifyToken }),
  });
  assert.equal(verifyRes.status, 200, 'Activation failed');

  // 登录普通用户
  const loginRegularRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: regularEmail,
      password: 'RegularPassword2026!',
    }),
  });
  assert.equal(loginRegularRes.status, 200, 'Regular login failed');
  regularUserCookie = extractSessionCookie(loginRegularRes);
  assert.ok(regularUserCookie, 'Regular session cookie must be set');

  // 0.2 登录管理员 (在 Step 2 中已初始化)
  const loginAdminRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@attackbox.org',
      password: 'AttackBoxAdmin2026!',
    }),
  });
  const loginAdminData = await loginAdminRes.json();
  assert.equal(loginAdminRes.status, 200, `Admin login failed: ${JSON.stringify(loginAdminData)}`);
  adminCookie = extractSessionCookie(loginAdminRes);
  assert.ok(adminCookie, 'Admin session cookie must be set');

  console.log('[PASS] Setup: Regular and Admin sessions established');
});

test('1. RBAC Security: Require Admin Verification', async () => {
  // 1.1 未登录请求返回 401
  const unauthRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ slug: 'unauth-test' }),
  });
  assert.equal(unauthRes.status, 401);

  // 1.2 普通用户请求返回 403
  const forbidRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: regularUserCookie,
    },
    body: JSON.stringify({ slug: 'forbid-test' }),
  });
  assert.equal(forbidRes.status, 403);

  console.log('[PASS] RBAC: 401 for unauthenticated, 403 for non-admin');
});

test('2. Admin Module Creation & List Query', async () => {
  const slug = `metasploit-e2e-${Date.now()}`;
  const createRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug,
      title: 'Metasploit 进阶渗透实战',
      summary: '从架构剖析到 Payload 生成与后渗透全流程',
      difficulty: 'intermediate',
    }),
  });
  const createData = await createRes.json();
  assert.equal(createRes.status, 200);
  assert.ok(createData.data.module.id);
  assert.equal(createData.data.draft.revisionNo, 1);
  assert.equal(createData.data.draft.lockVersion, 1);
  createdModuleId = createData.data.module.id;
  draftLockVersion = createData.data.draft.lockVersion;

  // 2.2 验证 Slug 冲突返回 409
  const dupRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug,
      title: '重复 Slug 模块',
    }),
  });
  assert.equal(dupRes.status, 409);

  // 2.3 获取模块列表，断言存在
  const listRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    headers: { Cookie: adminCookie },
  });
  const listData = await listRes.json();
  assert.equal(listRes.status, 200);
  const found = listData.data.find((m) => m.id === createdModuleId);
  assert.ok(found);
  assert.equal(found.slug, slug);
  assert.equal(found.draft.title, 'Metasploit 进阶渗透实战');

  console.log('[PASS] Module created and verified in list');
});

test('3. Module Draft Update & Optimistic Lock (409 Conflict)', async () => {
  // 3.1 正常更新草稿 (带正确的 lockVersion)
  const updateRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      title: 'Metasploit 进阶渗透实战 (已更新)',
      summary: '新增免杀测试章节',
      difficulty: 'advanced',
      lockVersion: draftLockVersion,
    }),
  });
  const updateData = await updateRes.json();
  assert.equal(updateRes.status, 200);
  assert.equal(updateData.data.lockVersion, 2);
  draftLockVersion = updateData.data.lockVersion;

  // 3.2 携带过期的旧 lockVersion: 1 再次更新，必须触发 409 Conflict
  const conflictRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      title: '并发冲突标题',
      lockVersion: 1, // 过期版本
    }),
  });
  assert.equal(conflictRes.status, 409);

  console.log('[PASS] Draft updated with lock_version increment, stale version triggers 409');
});

test('4. Chapters Creation in Module Draft', async () => {
  const chaptersToCreate = [
    { slug: 'msf-architecture', title: 'MSF 架构与模块剖析', access: 'public' },
    { slug: 'msf-payload-crafting', title: 'Payload 生成与免杀测试', access: 'authenticated' },
    { slug: 'msf-post-exploit', title: '后渗透与提权操作', access: 'authenticated' },
  ];

  for (let i = 0; i < chaptersToCreate.length; i++) {
    const ch = chaptersToCreate[i];
    const res = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}/chapters`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Cookie: adminCookie,
      },
      body: JSON.stringify(ch),
    });
    const data = await res.json();
    assert.equal(res.status, 200);
    assert.equal(data.data.position, i + 1);
    createdChapterIds.push(data.data.chapter.id);
  }

  assert.equal(createdChapterIds.length, 3);

  // 4.2 验证同模块下重复 Slug 返回 409
  const dupChRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}/chapters`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug: 'msf-architecture',
      title: '重复章节',
    }),
  });
  assert.equal(dupChRes.status, 409);

  console.log('[PASS] 3 chapters created with sequential positions (1, 2, 3)');
});

test('5. Chapter Content Draft & Quiz Questions Setting', async () => {
  const targetChapterId = createdChapterIds[1]; // 第 2 章

  const saveRes = await fetch(`${BASE_URL}/api/admin/chapters/${targetChapterId}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      moduleId: createdModuleId,
      title: 'Payload 生成与免杀测试 (精讲)',
      access: 'authenticated',
      bodyMarkdown: '# Payload 生成\n\n使用 `msfvenom` 工具生成各类 payload...',
      questions: [
        {
          position: 1,
          promptMarkdown: 'msfvenom 生成可执行文件的格式参数是？',
          normalization: 'trim',
          answer: '-f',
        },
        {
          position: 2,
          promptMarkdown: 'Meterpreter 中查看目标系统信息的命令是？',
          normalization: 'trim_casefold',
          answer: 'sysinfo',
        },
      ],
    }),
  });

  const saveData = await saveRes.json();
  assert.equal(saveRes.status, 200);
  assert.equal(saveData.data.questions.length, 2);
  assert.equal(saveData.data.questions[0].hasAnswer, true);
  assert.equal(saveData.data.questions[1].hasAnswer, true);
  // 确保没有返回原始 answerHmac
  assert.equal(saveData.data.questions[0].answerHmac, undefined);

  // 5.2 获取模块详情检查正文与题目回显
  const modDetailRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}`, {
    headers: { Cookie: adminCookie },
  });
  const modDetail = await modDetailRes.json();
  assert.equal(modDetailRes.status, 200);
  assert.equal(modDetail.data.chapters.length, 3);

  const ch2 = modDetail.data.chapters.find((c) => c.chapterId === targetChapterId);
  assert.ok(ch2);
  assert.equal(ch2.title, 'Payload 生成与免杀测试 (精讲)');
  assert.equal(ch2.questions.length, 2);

  console.log('[PASS] Chapter content & HMAC quiz questions configured and verified');
});

test('6. Chapters Reordering in Draft', async () => {
  // 原顺序 [ch0: pos 1, ch1: pos 2, ch2: pos 3]
  // 重新排序为 [ch2: pos 1, ch0: pos 2, ch1: pos 3]
  const reorderRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}/chapters/reorder`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      orders: [
        { chapterId: createdChapterIds[2], position: 1 },
        { chapterId: createdChapterIds[0], position: 2 },
        { chapterId: createdChapterIds[1], position: 3 },
      ],
    }),
  });
  assert.equal(reorderRes.status, 200);

  // 再次获取模块详情，验证章节编排列表的顺序
  const modDetailRes = await fetch(`${BASE_URL}/api/admin/modules/${createdModuleId}`, {
    headers: { Cookie: adminCookie },
  });
  const modDetail = await modDetailRes.json();
  assert.equal(modDetailRes.status, 200);

  const orderedChapterIds = modDetail.data.chapters.map((c) => c.chapterId);
  assert.deepEqual(orderedChapterIds, [
    createdChapterIds[2],
    createdChapterIds[0],
    createdChapterIds[1],
  ]);

  console.log('[PASS] Chapters reordered successfully to [ch2, ch0, ch1]');
});

test.after(async () => {
  console.log('--------------------------------------------------');
  console.log('AttackBox Admin Content Remote E2E Tests Complete!');
  console.log('--------------------------------------------------');
});
