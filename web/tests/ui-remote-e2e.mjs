// web/tests/ui-remote-e2e.mjs
import test from 'node:test';
import assert from 'node:assert/strict';

const BASE_URL = process.env.BASE_URL || 'http://77.67.1.106';

test('Remote UI End-to-End: Learn and Admin Pages accessibility', async () => {
  // 1. 验证学习平台首页返回 200 与关键 HTML 元素
  const learnRes = await fetch(`${BASE_URL}/learn`);
  assert.equal(learnRes.status, 200, '/learn should return 200 OK');
  const learnHtml = await learnRes.text();
  assert.ok(learnHtml.includes('AttackBox'), 'Must contain AttackBox branding');
  assert.ok(learnHtml.includes('让知识，成为你的能力') || learnHtml.includes('YOUR NEXT CHAPTER'), 'Must render learning page title');

  // 2. 验证管理后台入口返回 200
  const adminRes = await fetch(`${BASE_URL}/admin/modules`);
  assert.equal(adminRes.status, 200, '/admin/modules should return 200 OK');
  const adminHtml = await adminRes.text();
  assert.ok(
    adminHtml.includes('admin-shell') ||
    adminHtml.includes('正在验证管理员身份') ||
    adminHtml.includes('AttackBox Admin') ||
    adminHtml.includes('学习模块') ||
    adminHtml.includes('需要管理员权限'),
    'Must render admin shell'
  );
});

test('Remote UI End-to-End: Admin Content Management & Reader Flow', async () => {
  // 1. 管理员登录
  const loginRes = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@attackbox.org',
      password: 'AttackBoxAdmin2026!'
    })
  });

  assert.equal(loginRes.status, 200, 'Admin login must succeed');
  const cookie = loginRes.headers.get('set-cookie');
  assert.ok(cookie, 'Must return session cookie');

  const authHeaders = {
    'Content-Type': 'application/json',
    'Cookie': cookie
  };

  // 2. 创建新学习模块
  const testSlug = `e2e-ui-mod-${Date.now()}`;
  const createModRes = await fetch(`${BASE_URL}/api/admin/modules`, {
    method: 'POST',
    headers: authHeaders,
    body: JSON.stringify({
      title: 'UI 端到端自动化验证模块',
      slug: testSlug,
      summary: '自动化验证学习平台与管理后台完整发布流程',
      difficulty: 'beginner'
    })
  });

  assert.equal(createModRes.status, 200, 'Create module must return 200');
  const modData = await createModRes.json();
  const moduleId = modData.data.module.id;
  assert.ok(moduleId, 'Module ID must be present');

  // 3. 为模块创建两章
  const ch1Res = await fetch(`${BASE_URL}/api/admin/modules/${moduleId}/chapters`, {
    method: 'POST',
    headers: authHeaders,
    body: JSON.stringify({
      title: '第一章：基础认识与环境检查',
      slug: 'intro',
      access: 'public',
      bodyMarkdown: '# 第一章：基础认识\n\n欢迎学习安全知识。\n\n```bash\npwd\necho "ready"\n```\n\n> [!NOTE]\n> 本章目标：确认终端环境。'
    })
  });
  assert.equal(ch1Res.status, 200, 'Create chapter 1 must return 200');
  const ch1Data = await ch1Res.json();
  const ch1Id = ch1Data.data.chapter.id;

  const ch2Res = await fetch(`${BASE_URL}/api/admin/modules/${moduleId}/chapters`, {
    method: 'POST',
    headers: authHeaders,
    body: JSON.stringify({
      title: '第二章：实操命令与演练',
      slug: 'practice',
      access: 'public',
      bodyMarkdown: '# 第二章：实操命令\n\n执行测试脚本并分析输出。\n\n```bash\nls -la\n```'
    })
  });
  assert.equal(ch2Res.status, 200, 'Create chapter 2 must return 200');
  const ch2Data = await ch2Res.json();
  const ch2Id = ch2Data.data.chapter.id;

  // 4. 为章节配置测试练习题及标准答案
  const updateCh1Res = await fetch(`${BASE_URL}/api/admin/chapters/${ch1Id}/draft`, {
    method: 'PUT',
    headers: authHeaders,
    body: JSON.stringify({
      moduleId,
      title: '第一章：基础认识与环境检查',
      access: 'public',
      bodyMarkdown: '# 第一章：基础认识\n\n欢迎学习安全知识。\n\n```bash\npwd\necho "ready"\n```\n\n> [!NOTE]\n> 本章目标：确认终端环境。',
      questions: [
        {
          promptMarkdown: '查看当前目录应使用什么命令？',
          answer: 'pwd',
          errorHint: '它是 print working directory 的缩写',
          normalization: 'trim'
        }
      ]
    })
  });
  assert.equal(updateCh1Res.status, 200, 'Update chapter 1 draft must succeed');

  const updateCh2Res = await fetch(`${BASE_URL}/api/admin/chapters/${ch2Id}/draft`, {
    method: 'PUT',
    headers: authHeaders,
    body: JSON.stringify({
      moduleId,
      title: '第二章：实操命令与演练',
      access: 'public',
      bodyMarkdown: '# 第二章：实操命令\n\n执行测试脚本并分析输出。\n\n```bash\nls -la\n```',
      questions: [
        {
          promptMarkdown: '列出文件详细信息应使用什么命令？',
          answer: 'ls -la',
          errorHint: '包含 -l 与 -a 选项',
          normalization: 'collapse_whitespace'
        }
      ]
    })
  });
  assert.equal(updateCh2Res.status, 200, 'Update chapter 2 draft must succeed');

  // 5. 发布模块
  const publishRes = await fetch(`${BASE_URL}/api/admin/modules/${moduleId}/publish`, {
    method: 'POST',
    headers: authHeaders,
    body: JSON.stringify({})
  });
  assert.equal(publishRes.status, 200, 'Publish module must return 200');

  // 6. 验证前台学员视角访问
  // 6.1 模块目录在 API 中可见
  const listRes = await fetch(`${BASE_URL}/api/modules`);
  assert.equal(listRes.status, 200);
  const listJson = await listRes.json();
  const found = listJson.data.find(m => m.slug === testSlug);
  assert.ok(found, 'Published module must appear in /api/modules list');
  assert.equal(found.chapterCount, 2, 'Must report 2 chapters');

  // 6.2 模块大纲页 SSR 渲染返回 200
  const modPageRes = await fetch(`${BASE_URL}/learn/modules/${testSlug}`);
  assert.equal(modPageRes.status, 200, 'Module roadmap page must return 200');
  const modPageHtml = await modPageRes.text();
  assert.ok(modPageHtml.includes('UI 端到端自动化验证模块'), 'Roadmap HTML must include module title');
  assert.ok(modPageHtml.includes('第一章：基础认识与环境检查'), 'Roadmap HTML must include chapter 1');
  assert.ok(modPageHtml.includes('第二章：实操命令与演练'), 'Roadmap HTML must include chapter 2');

  // 6.3 第一章正文阅读页 SSR 渲染返回 200
  const chPageRes = await fetch(`${BASE_URL}/learn/modules/${testSlug}/intro`);
  assert.equal(chPageRes.status, 200, 'Chapter page must return 200');
  const chPageHtml = await chPageRes.text();
  assert.ok(chPageHtml.includes('第一章：基础认识与环境检查'), 'Chapter HTML must include chapter title');
  assert.ok(chPageHtml.includes('pwd'), 'Chapter HTML must include code block content');
  assert.ok(chPageHtml.includes('查看当前目录应使用什么命令？'), 'Chapter HTML must include quiz prompt');

  // 7. 测试下架恢复
  const unpublishRes = await fetch(`${BASE_URL}/api/admin/modules/${moduleId}/unpublish`, {
    method: 'POST',
    headers: authHeaders,
    body: JSON.stringify({})
  });
  assert.equal(unpublishRes.status, 200, 'Unpublish module must succeed');

  const afterUnpublishRes = await fetch(`${BASE_URL}/api/modules/${testSlug}`);
  assert.equal(afterUnpublishRes.status, 404, 'Unpublished module must return 404 to public readers');
});
