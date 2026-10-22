// web/tests/reader-content-remote-e2e.mjs
// AttackBox Phase 2 Step 3.4: Reader Content & Access Control Remote E2E Tests
import test from 'node:test';
import assert from 'node:assert/strict';

const BASE_URL = process.env.BASE_URL || 'http://127.0.0.1:3000';

console.log('--------------------------------------------------');
console.log('Starting AttackBox Reader Content Remote E2E Tests');
console.log(`Base URL: ${BASE_URL}`);
console.log('--------------------------------------------------');

let adminCookie = '';
let userCookie = '';
const testSlug = `reader-mod-${Date.now()}`;
let testModuleId = '';

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

test('0. Setup: Users & Module Creation (with Public and Authenticated chapters)', async () => {
  // 0.1 管理员登录
  const loginAdminRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@attackbox.org',
      password: 'AttackBoxAdmin2026!',
    }),
  });
  assert.equal(loginAdminRes.status, 200);
  adminCookie = extractSessionCookie(loginAdminRes);

  // 0.2 注册并激活普通用户
  const userEmail = `reader_${Date.now()}@example.com`;
  const regRes = await fetch(`${BASE_URL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: userEmail,
      displayName: 'Reader Student',
      password: 'StudentPassword2026!',
    }),
  });
  const regData = await regRes.json();
  assert.equal(regRes.status, 200);

  await fetch(`${BASE_URL}/api/auth/verify-email`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: regData.verifyToken }),
  });

  const loginUserRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: userEmail,
      password: 'StudentPassword2026!',
    }),
  });
  userCookie = extractSessionCookie(loginUserRes);
  assert.ok(userCookie);

  // 0.3 管理员创建模块
  const modRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug: testSlug,
      title: '网络渗透测试基础课程',
      summary: '从公网侦察到漏洞利用的实战指南',
      difficulty: 'beginner',
    }),
  });
  const modData = await modRes.json();
  testModuleId = modData.data.module.id;

  // 0.4 创建公开章节
  const ch1Res = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/chapters`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug: 'recon-basics',
      title: '第一章：网络侦察基础',
      access: 'public',
      bodyMarkdown: '# 网络侦察\n\nNmap 扫描参数 `-sS` 与 `-sV` 的应用...',
    }),
  });
  assert.equal(ch1Res.status, 200);

  // 0.5 创建受限章节（带测验题）
  const ch2Res = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/chapters`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      slug: 'vuln-exploit',
      title: '第二章：漏洞利用进阶 (会员/登录可见)',
      access: 'authenticated',
      bodyMarkdown: '# 漏洞利用\n\nMetasploit exploit 模块加载...',
    }),
  });
  const ch2Data = await ch2Res.json();
  assert.equal(ch2Res.status, 200);

  // 设置第二章的题目与答案
  await fetch(`${BASE_URL}/api/admin/chapters/${ch2Data.data.chapter.id}/draft`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
    body: JSON.stringify({
      moduleId: testModuleId,
      title: '第二章：漏洞利用进阶 (会员/登录可见)',
      access: 'authenticated',
      bodyMarkdown: '# 漏洞利用\n\nMetasploit exploit 模块加载...',
      questions: [
        {
          position: 1,
          promptMarkdown: '在 Metasploit 中设置攻击目标的命令关键字是？',
          normalization: 'trim_casefold',
          answer: 'rhosts',
        },
      ],
    }),
  });

  console.log('[PASS] Setup complete. Test module created with public and authenticated chapters.');
});

test('1. Unreleased Module Is Hidden from Reader Endpoints (404)', async () => {
  // 1.1 列表不包含未发布模块
  const listRes = await fetch(`${BASE_URL}/api/modules`);
  const listData = await listRes.json();
  assert.equal(listRes.status, 200);
  const found = listData.data.find((m) => m.slug === testSlug);
  assert.equal(found, undefined, 'Unreleased module must not appear in reader list');

  // 1.2 直接访问返回 404
  const modRes = await fetch(`${BASE_URL}/api/modules/${testSlug}`);
  assert.equal(modRes.status, 404);

  console.log('[PASS] Unreleased module strictly hidden from public list and direct slug URL');
});

test('2. Released Module Appears in Reader Catalog with Outline & Lock State', async () => {
  // 2.1 管理员发布模块
  const pubRes = await fetch(`${BASE_URL}/api/admin/modules/${testModuleId}/publish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: adminCookie,
    },
  });
  assert.equal(pubRes.status, 200);

  // 2.2 读者列表可见
  const listRes = await fetch(`${BASE_URL}/api/modules`);
  const listData = await listRes.json();
  assert.equal(listRes.status, 200);
  const found = listData.data.find((m) => m.slug === testSlug);
  assert.ok(found);
  assert.equal(found.title, '网络渗透测试基础课程');
  assert.equal(found.chapterCount, 2);

  // 2.3 访客读取大纲：检查受限锁状态
  const guestOutlineRes = await fetch(`${BASE_URL}/api/modules/${testSlug}`);
  const guestOutlineData = await guestOutlineRes.json();
  assert.equal(guestOutlineRes.status, 200);
  assert.equal(guestOutlineData.data.chapters.length, 2);

  const publicCh = guestOutlineData.data.chapters.find((c) => c.slug === 'recon-basics');
  const authCh = guestOutlineData.data.chapters.find((c) => c.slug === 'vuln-exploit');
  assert.equal(publicCh.isLocked, false);
  assert.equal(authCh.isLocked, true); // 访客状态下受限章节显示锁定

  // 2.4 已登录学生读取大纲：受限章节解锁
  const userOutlineRes = await fetch(`${BASE_URL}/api/modules/${testSlug}`, {
    headers: { Cookie: userCookie },
  });
  const userOutlineData = await userOutlineRes.json();
  assert.equal(userOutlineRes.status, 200);
  const authChUser = userOutlineData.data.chapters.find((c) => c.slug === 'vuln-exploit');
  assert.equal(authChUser.isLocked, false); // 登录状态下已解锁

  console.log('[PASS] Reader outline correctly reports locked/unlocked based on authentication');
});

test('3. Reading Public Chapter Succeeds for Guests', async () => {
  const readRes = await fetch(`${BASE_URL}/api/modules/${testSlug}/chapters/recon-basics`);
  const readData = await readRes.json();
  assert.equal(readRes.status, 200);
  assert.equal(readData.data.chapter.title, '第一章：网络侦察基础');
  assert.match(readData.data.chapter.bodyMarkdown, /Nmap 扫描参数/);

  console.log('[PASS] Public chapter readable by guest without session');
});

test('4. Reading Authenticated Chapter Blocks Guests (401) and Succeeds for Logged-In User', async () => {
  // 4.1 访客请求受限章节返回 401
  const guestRes = await fetch(`${BASE_URL}/api/modules/${testSlug}/chapters/vuln-exploit`);
  assert.equal(guestRes.status, 401);

  // 4.2 登录学生请求受限章节返回 200
  const userRes = await fetch(`${BASE_URL}/api/modules/${testSlug}/chapters/vuln-exploit`, {
    headers: { Cookie: userCookie },
  });
  const userData = await userRes.json();
  assert.equal(userRes.status, 200);
  assert.equal(userData.data.chapter.title, '第二章：漏洞利用进阶 (会员/登录可见)');
  assert.match(userData.data.chapter.bodyMarkdown, /Metasploit exploit/);

  // 4.3 校验题目白名单：不暴露 answerHmac
  assert.equal(userData.data.questions.length, 1);
  assert.equal(userData.data.questions[0].promptMarkdown, '在 Metasploit 中设置攻击目标的命令关键字是？');
  assert.equal(userData.data.questions[0].answerHmac, undefined);
  assert.equal(userData.data.questions[0].answer, undefined);

  console.log('[PASS] Authenticated chapter protected by 401, sanitized questions delivered to logged-in user');
});

test.after(async () => {
  console.log('--------------------------------------------------');
  console.log('AttackBox Reader Content Remote E2E Tests Complete!');
  console.log('--------------------------------------------------');
});
