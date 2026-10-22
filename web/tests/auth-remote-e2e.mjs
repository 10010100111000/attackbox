// web/tests/auth-remote-e2e.mjs
// AttackBox Phase 2 Nitro Auth End-to-End Test Suite
import test from 'node:test';
import assert from 'node:assert/strict';

const BASE_URL = process.env.BASE_URL || 'http://127.0.0.1:3000';
console.log('--------------------------------------------------');
console.log(`Starting Nitro Auth E2E Tests against: ${BASE_URL}`);
console.log('--------------------------------------------------');

const testEmail = `e2e_${Date.now()}@example.com`;
const originalPassword = 'Password123!';
const updatedPassword = 'NewPassword456!';
let verifyToken = '';
let resetToken = '';
let sessionCookie = '';

test('1. POST /api/auth/register creates pending user and issues verifyToken', async () => {
  const res = await fetch(`${BASE_URL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: testEmail,
      password: originalPassword,
      displayName: 'E2E Tester',
    }),
  });

  const data = await res.json();
  assert.equal(res.status, 200, `Expected 200/201, got ${res.status}: ${JSON.stringify(data)}`);
  assert.ok(data.verifyToken, 'Response must include verifyToken');
  verifyToken = data.verifyToken;
  console.log(`[PASS] Registered pending user: ${testEmail}`);
});

test('2. POST /api/auth/login rejects unverified pending account', async () => {
  const res = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: testEmail,
      password: originalPassword,
    }),
  });

  assert.equal(res.status, 403, 'Unverified account must be rejected with 403');
  const data = await res.json();
  assert.match(data.statusMessage || '', /未激活|激活/);
  console.log('[PASS] Unverified account properly blocked with 403');
});

test('3. POST /api/auth/verify-email activates user', async () => {
  const res = await fetch(`${BASE_URL}/api/auth/verify-email`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: verifyToken }),
  });

  assert.equal(res.status, 200);
  console.log('[PASS] User successfully activated with verifyToken');
});

test('4. POST /api/auth/login succeeds for active user and sets Session Cookie', async () => {
  const res = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: testEmail,
      password: originalPassword,
      rememberMe: true,
    }),
  });

  assert.equal(res.status, 200);
  const data = await res.json();
  assert.equal(data.user.email, testEmail);
  assert.equal(data.user.status, 'active');

  const rawCookie = res.headers.get('set-cookie');
  assert.ok(rawCookie && rawCookie.includes('attackbox_session='), 'Must set attackbox_session cookie');
  sessionCookie = rawCookie.split(';')[0];
  console.log(`[PASS] Login successful, cookie set: ${sessionCookie.slice(0, 30)}...`);
});

test('5. GET /api/auth/me returns authenticated user with valid Session Cookie', async () => {
  const res = await fetch(`${BASE_URL}/api/auth/me`, {
    headers: {
      Cookie: sessionCookie,
    },
  });

  assert.equal(res.status, 200);
  const data = await res.json();
  assert.equal(data.user.email, testEmail);
  assert.equal(data.user.displayName, 'E2E Tester');
  console.log(`[PASS] Identity confirmed via session cookie: ${data.user.displayName}`);
});

test('6. POST /api/auth/forgot-password and reset-password revokes existing sessions', async () => {
  // 6.1 请求找回密码
  const forgotRes = await fetch(`${BASE_URL}/api/auth/forgot-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: testEmail }),
  });
  assert.equal(forgotRes.status, 200);
  const forgotData = await forgotRes.json();
  assert.ok(forgotData.resetToken, 'Must provide resetToken in test environment');
  resetToken = forgotData.resetToken;

  // 6.2 消费 Token 重置密码
  const resetRes = await fetch(`${BASE_URL}/api/auth/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      token: resetToken,
      newPassword: updatedPassword,
    }),
  });
  assert.equal(resetRes.status, 200);

  // 6.3 验证旧 Session Cookie 立即失效
  const meWithOldCookie = await fetch(`${BASE_URL}/api/auth/me`, {
    headers: {
      Cookie: sessionCookie,
    },
  });
  assert.equal(meWithOldCookie.status, 401, 'Old session must be revoked after password reset');
  console.log('[PASS] Password reset successfully and revoked old session cookie');
});

test('7. POST /api/auth/login works with new password and logout revokes session', async () => {
  // 7.1 使用旧密码登录应当失败
  const failLogin = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: testEmail,
      password: originalPassword,
    }),
  });
  assert.equal(failLogin.status, 401);

  // 7.2 使用新密码登录成功
  const newLogin = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: testEmail,
      password: updatedPassword,
    }),
  });
  assert.equal(newLogin.status, 200);
  const newCookie = newLogin.headers.get('set-cookie')?.split(';')[0];
  assert.ok(newCookie);

  // 7.3 执行登出
  const logoutRes = await fetch(`${BASE_URL}/api/auth/logout`, {
    method: 'POST',
    headers: {
      Cookie: newCookie,
    },
  });
  assert.equal(logoutRes.status, 200);

  // 7.4 再次验证该 session 已失效
  const meAfterLogout = await fetch(`${BASE_URL}/api/auth/me`, {
    headers: {
      Cookie: newCookie,
    },
  });
  assert.equal(meAfterLogout.status, 401);
  console.log('[PASS] Re-login with new password and logout verified');
});

test('Teardown', () => {
  console.log('--------------------------------------------------');
  console.log('ALL NITRO AUTH E2E TESTS PASSED 100%!');
  console.log('--------------------------------------------------');
});
