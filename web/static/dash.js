/**
 * AttackBox 控制台核心业务逻辑 (认证、导航、管理员运维、状态维护)
 */

const state = {
  user: null,
  boxId: '',
  activeTab: 'workspace',
  stageMode: 'cli',
  cliRunning: false,
  guiRunning: false,
  envRunning: false,
  expiresIn: 0,
  bannerDismissed: false,
  broadcast: { enabled: false, message: '' },
  users: []
};

// 暴露到全局供调试与模块读取
window.state = state;

function showToast(msg) {
  const container = document.getElementById('toast-container');
  if (!container) return;
  const el = document.createElement('div');
  el.className = 'bg-zinc-900 text-white px-3 py-2 rounded-lg shadow text-xs font-mono font-medium';
  el.textContent = msg;
  container.appendChild(el);
  setTimeout(() => el.remove(), 2500);
}

function escapeHtml(str) {
  if (str === null || str === undefined) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// ==================== 登录 / 注册 Tab 切换与提交 ====================

let authMode = 'login';

function switchAuthTab(mode) {
  authMode = mode;
  const isLogin = mode === 'login';
  document.getElementById('tab-login').className = isLogin
    ? 'py-1.5 text-xs font-mono font-medium rounded-md bg-white text-zinc-900 shadow-sm'
    : 'py-1.5 text-xs font-mono font-medium rounded-md text-zinc-500';
  document.getElementById('tab-register').className = !isLogin
    ? 'py-1.5 text-xs font-mono font-medium rounded-md bg-white text-zinc-900 shadow-sm'
    : 'py-1.5 text-xs font-mono font-medium rounded-md text-zinc-500';
  document.getElementById('auth-submit-btn').textContent = isLogin ? 'ENTER' : 'REGISTER & ENTER';
}

async function handleAuthSubmit(e) {
  e.preventDefault();
  const username = document.getElementById('auth-username').value.trim();
  const password = document.getElementById('auth-password').value;
  if (!username || !password) return;

  const endpoint = authMode === 'login' ? '/api/login' : '/api/register';
  try {
    const resp = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '登录/注册失败，请检查账号密码');
      return;
    }
    state.user = data.user;
    state.activeTab = 'workspace';
    showToast(authMode === 'login' ? '登录成功' : '注册成功并登入');
    initApp();
  } catch (err) {
    showToast('请求失败: ' + err.message);
  }
}

async function logout() {
  try {
    await fetch('/api/logout', { method: 'POST' });
  } catch (e) {}
  state.user = null;
  state.activeTab = 'workspace';
  if (countdownTimer) clearInterval(countdownTimer);
  document.getElementById('app-view').classList.add('hidden');
  document.getElementById('auth-view').classList.remove('hidden');
  showToast('已安全退出');
}

// ==================== 独立窗口唤起 ====================

function openCliWindow() {
  if (state && !state.cliRunning) {
    try {
      fetch('/api/env/cli/toggle', { method: 'POST' }).catch(() => {});
    } catch (e) {}
    state.cliRunning = true;
    state.envRunning = true;
    renderEnvStatus();
  }
  window.open('/term', '_blank');
  showToast('正在打开 TERMINAL 空间...');
}

function openGuiWindow() {
  if (state && !state.guiRunning) {
    try {
      fetch('/api/env/gui/toggle', { method: 'POST' }).catch(() => {});
    } catch (e) {}
    state.guiRunning = true;
    state.envRunning = true;
    renderEnvStatus();
  }
  window.open('/gui', '_blank');
  showToast('正在打开 DESKTOP 桌面...');
}

// ==================== 倒计时与沙箱状态维护 ====================

let countdownTimer = null;

function startLeaseCountdown(seconds) {
  if (countdownTimer) clearInterval(countdownTimer);
  let remaining = typeof seconds === 'number' ? seconds : 0;

  function updateDisplay() {
    const el = document.getElementById('lease-countdown-text');
    if (!el) return;
    if (!state.envRunning || remaining <= 0) {
      el.textContent = 'STANDBY';
      el.className = 'font-semibold text-zinc-500 tracking-wide';
      return;
    }
    el.className = 'font-semibold text-zinc-900 tracking-wide';
    const h = String(Math.floor(remaining / 3600)).padStart(2, '0');
    const m = String(Math.floor((remaining % 3600) / 60)).padStart(2, '0');
    const s = String(remaining % 60).padStart(2, '0');
    el.textContent = `${h}:${m}:${s}`;
  }

  updateDisplay();
  if (state.envRunning && remaining > 0) {
    countdownTimer = setInterval(() => {
      remaining--;
      updateDisplay();
      if (remaining <= 0) {
        clearInterval(countdownTimer);
        initApp();
      }
    }, 1000);
  }
}

function renderEnvStatus() {
  const isRunning = !!(state.envRunning || state.cliRunning || state.guiRunning);
  state.envRunning = isRunning;

  const badge = document.getElementById('env-status-badge');
  const badgeDot = document.getElementById('env-status-dot');
  const badgeText = document.getElementById('env-status-text');
  const btnDestroy = document.getElementById('btn-destroy-env');

  if (!badge) return;

  if (isRunning) {
    badge.className = 'px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200 flex items-center gap-1.5 transition-colors';
    badgeDot.className = 'w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse';
    badgeText.textContent = state.boxId ? `#${state.boxId} ONLINE` : 'ONLINE';
    if (btnDestroy) btnDestroy.classList.remove('hidden');
  } else {
    badge.className = 'px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-zinc-100 text-zinc-500 border border-zinc-200 flex items-center gap-1.5';
    badgeDot.className = 'w-1.5 h-1.5 rounded-full bg-zinc-400';
    badgeText.textContent = 'STANDBY';
    if (btnDestroy) btnDestroy.classList.add('hidden');
  }
}

async function destroyEnv() {
  if (!confirm('确定要彻底释放当前渗透沙箱吗？未保存的数据将被清空，所有正在运行的会话将被终止。')) return;
  const btn = document.getElementById('btn-destroy-env');
  if (btn) {
    btn.disabled = true;
    btn.classList.add('opacity-70', 'cursor-not-allowed');
  }

  try {
    const resp = await fetch('/api/env/destroy', { method: 'POST' });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '释放失败');
      return;
    }
    state.cliRunning = false;
    state.guiRunning = false;
    state.envRunning = false;
    state.expiresIn = 0;
    if (countdownTimer) clearInterval(countdownTimer);
    startLeaseCountdown(0);
    showToast('沙箱环境已安全销毁释放');
  } catch (e) {
    showToast('释放异常: ' + e.message);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.classList.remove('opacity-70', 'cursor-not-allowed');
    }
    renderEnvStatus();
  }
}

// ==================== 顶部导航切换 ====================

function switchTab(tab) {
  if (state.user && state.user.role !== 'admin' && tab.startsWith('admin-')) {
    tab = 'workspace';
  }
  state.activeTab = tab;

  ['workspace', 'account', 'admin-vps', 'admin-users', 'admin-broadcast'].forEach(id => {
    const el = document.getElementById(`view-${id}`);
    if (el) el.classList.toggle('hidden', id !== tab);
  });

  document.querySelectorAll('.nav-tab').forEach(btn => {
    const active = btn.getAttribute('data-tab') === tab;
    btn.className = `nav-tab ${btn.classList.contains('admin-only') ? 'admin-only' : ''} px-2.5 py-1.5 rounded-md text-xs font-mono font-medium flex items-center gap-1 transition-all cursor-pointer ${
      active ? 'bg-zinc-900 text-white shadow-sm' : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
    }`;
    if (state.user && state.user.role !== 'admin' && btn.classList.contains('admin-only')) {
      btn.classList.add('hidden');
    }
  });

  if (tab === 'account') {
    renderAccountTab();
  }
  if (tab === 'admin-vps') fetchMetrics();
  if (tab === 'admin-users') fetchUsers();
}

function renderAccountTab() {
  if (!state.user) return;
  const usernameEl = document.getElementById('account-username-text');
  const leaseEl = document.getElementById('account-lease-text');
  const roleBadge = document.getElementById('account-role-badge');
  if (usernameEl) usernameEl.textContent = state.user.username;
  if (leaseEl) leaseEl.textContent = state.boxId ? `#${state.boxId}` : 'STANDBY';
  if (roleBadge) roleBadge.textContent = state.user.role === 'admin' ? 'ADMIN' : 'USER';
}

async function handleAccountPasswordChange(e) {
  e.preventDefault();
  const oldPassword = document.getElementById('acc-old-pass').value;
  const newPassword = document.getElementById('acc-new-pass').value;
  const confirmPassword = document.getElementById('acc-confirm-pass').value;
  if (!oldPassword || !newPassword) return;

  if (newPassword.length < 6) {
    showToast('新密码长度至少 6 位');
    return;
  }
  if (newPassword !== confirmPassword) {
    showToast('两次输入的新密码不一致');
    return;
  }

  const btn = document.getElementById('btn-update-pass');
  btn.disabled = true;
  btn.textContent = 'UPDATING...';
  try {
    const resp = await fetch('/api/user/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ oldPassword, newPassword })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '修改密码失败');
      return;
    }
    showToast('安全凭据已成功更新');
    document.getElementById('acc-old-pass').value = '';
    document.getElementById('acc-new-pass').value = '';
    document.getElementById('acc-confirm-pass').value = '';
  } catch (err) {
    showToast('请求异常: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'UPDATE PASSWORD';
  }
}

// ==================== 管理员操作：指标与用户 CRUD ====================

async function fetchMetrics() {
  if (!state.user || state.user.role !== 'admin') return;
  try {
    const resp = await fetch('/api/admin/metrics');
    if (!resp.ok) return;
    const d = await resp.json();
    document.getElementById('admin-cpu-text').textContent = d.cpuPercent + '%';
    document.getElementById('admin-cpu-bar').style.width = Math.min(d.cpuPercent, 100) + '%';
    document.getElementById('admin-ram-text').textContent = `${d.memUsedGB} GB / ${d.memTotalGB} GB`;
    document.getElementById('admin-ram-bar').style.width = Math.min((d.memUsedGB / d.memTotalGB) * 100, 100) + '%';
    document.getElementById('admin-disk-text').textContent = `${d.diskUsedGB} GB / ${d.diskTotalGB} GB`;
    document.getElementById('admin-boxes-count').textContent = `运行中沙箱: ${d.runningBoxes} 槽位`;
  } catch (e) {}
}

async function fetchUsers() {
  if (!state.user || state.user.role !== 'admin') return;
  try {
    const resp = await fetch('/api/admin/users');
    if (!resp.ok) return;
    state.users = await resp.json();
    renderUsersTable();
  } catch (e) {}
}

function renderUsersTable() {
  const tbody = document.getElementById('admin-users-tbody');
  if (!tbody) return;
  tbody.innerHTML = '';
  state.users.forEach(u => {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-zinc-50/50';
    tr.innerHTML = `
      <td class="py-2.5 px-4 font-medium text-zinc-900">${escapeHtml(u.username)}</td>
      <td class="py-2.5 px-4">
        <span class="px-1.5 py-0.5 rounded text-[10px] ${u.role === 'admin' ? 'bg-zinc-900 text-white' : 'bg-zinc-100 text-zinc-700'}">${escapeHtml(u.role)}</span>
      </td>
      <td class="py-2.5 px-4">
        <span class="px-1.5 py-0.5 rounded text-[10px] ${u.active ? 'bg-zinc-100 text-zinc-800' : 'bg-zinc-200 text-zinc-400'}">${u.active ? 'ACTIVE' : 'DISABLED'}</span>
      </td>
      <td class="py-2.5 px-4 text-zinc-400 text-[11px]">${escapeHtml(u.created_at)}</td>
      <td class="py-2.5 px-4 text-right space-x-2">
        <button onclick="toggleUserActive('${escapeHtml(u.username)}')" class="text-zinc-600 hover:text-zinc-900 text-xs cursor-pointer">${u.active ? '禁用' : '解禁'}</button>
        <button onclick="openResetPassModal('${escapeHtml(u.username)}')" class="text-zinc-600 hover:text-zinc-900 text-xs cursor-pointer">改密</button>
        ${u.username !== state.user.username ? `<button onclick="deleteUser('${escapeHtml(u.username)}')" class="text-zinc-400 hover:text-zinc-900 text-xs cursor-pointer">删除</button>` : ''}
      </td>
    `;
    tbody.appendChild(tr);
  });
}

function openCreateUserModal() {
  document.getElementById('modal-title').textContent = 'NEW USER';
  document.getElementById('modal-body').innerHTML = `
    <form onsubmit="handleCreateUser(event)" class="space-y-3 text-xs font-mono">
      <div>
        <label class="block font-medium text-zinc-700 mb-1">USERNAME</label>
        <input id="new-user-name" type="text" required placeholder="3-32位字母数字" class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div>
        <label class="block font-medium text-zinc-700 mb-1">PASSWORD</label>
        <input id="new-user-pass" type="password" required minlength="6" placeholder="至少6位" class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div>
        <label class="block font-medium text-zinc-700 mb-1">ROLE</label>
        <select id="new-user-role" class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900">
          <option value="user">USER (普通学生)</option>
          <option value="admin">ADMIN (管理员)</option>
        </select>
      </div>
      <div class="flex justify-end gap-2 pt-2">
        <button type="button" onclick="closeModal()" class="px-3 py-1.5 rounded-lg text-zinc-600 hover:bg-zinc-100 cursor-pointer">CANCEL</button>
        <button type="submit" class="px-3 py-1.5 rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 cursor-pointer">CREATE</button>
      </div>
    </form>
  `;
  document.getElementById('modal-overlay').classList.remove('hidden');
}

async function handleCreateUser(e) {
  e.preventDefault();
  const username = document.getElementById('new-user-name').value.trim();
  const password = document.getElementById('new-user-pass').value;
  const role = document.getElementById('new-user-role').value;
  try {
    const resp = await fetch('/api/admin/users/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, role })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '创建失败');
      return;
    }
    closeModal();
    fetchUsers();
    showToast(`用户【${username}】创建成功`);
  } catch (err) {
    showToast('请求异常: ' + err.message);
  }
}

function openResetPassModal(username) {
  document.getElementById('modal-title').textContent = `RESET PASSWORD: ${escapeHtml(username)}`;
  document.getElementById('modal-body').innerHTML = `
    <form onsubmit="handleResetPass(event, '${escapeHtml(username)}')" class="space-y-3 text-xs font-mono">
      <div>
        <label class="block font-medium text-zinc-700 mb-1">NEW PASSWORD</label>
        <input id="reset-user-pass" type="password" required minlength="6" placeholder="输入至少6位新密码" class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div class="flex justify-end gap-2 pt-2">
        <button type="button" onclick="closeModal()" class="px-3 py-1.5 rounded-lg text-zinc-600 hover:bg-zinc-100 cursor-pointer">CANCEL</button>
        <button type="submit" class="px-3 py-1.5 rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 cursor-pointer">RESET</button>
      </div>
    </form>
  `;
  document.getElementById('modal-overlay').classList.remove('hidden');
}

async function handleResetPass(e, username) {
  e.preventDefault();
  const newPassword = document.getElementById('reset-user-pass').value;
  try {
    const resp = await fetch('/api/admin/users/reset-pass', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, newPassword })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '重置失败');
      return;
    }
    closeModal();
    showToast(`用户【${username}】密码已重置`);
  } catch (err) {
    showToast('请求异常: ' + err.message);
  }
}

async function toggleUserActive(username) {
  try {
    const resp = await fetch('/api/admin/users/toggle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '切换失败');
      return;
    }
    fetchUsers();
    showToast('用户状态已更新');
  } catch (err) {
    showToast('请求异常: ' + err.message);
  }
}

async function deleteUser(username) {
  if (!confirm(`确定要彻底删除用户【${username}】吗？`)) return;
  try {
    const resp = await fetch('/api/admin/users/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '删除失败');
      return;
    }
    fetchUsers();
    showToast(`用户【${username}】已删除`);
  } catch (err) {
    showToast('请求异常: ' + err.message);
  }
}

// ==================== 全局广播通知 ====================

async function saveBroadcast(e) {
  e.preventDefault();
  const enabled = document.getElementById('broadcast-enabled').checked;
  const message = document.getElementById('broadcast-msg').value.trim();
  try {
    await fetch('/api/admin/broadcast', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled, message })
    });
    state.broadcast = { enabled, message };
    state.bannerDismissed = false;
    renderBanner();
    showToast('全局通知已更新并发布');
  } catch (err) {
    showToast('发布失败');
  }
}

function renderBanner() {
  const banner = document.getElementById('global-banner');
  if (state.broadcast && state.broadcast.enabled && !state.bannerDismissed && state.broadcast.message) {
    banner.classList.remove('hidden');
    document.getElementById('banner-text').textContent = state.broadcast.message;
  } else {
    banner.classList.add('hidden');
  }
}

function dismissBanner() {
  state.bannerDismissed = true;
  renderBanner();
}

function closeModal() {
  document.getElementById('modal-overlay').classList.add('hidden');
}

function copyIp(ip) {
  navigator.clipboard.writeText(ip).then(
    () => showToast('已复制: ' + ip),
    () => showToast('复制失败，请手动复制')
  );
}

// ==================== 应用初始化 ====================

async function initApp() {
  try {
    const resp = await fetch('/api/me');
    if (!resp.ok) {
      document.getElementById('auth-view').classList.remove('hidden');
      document.getElementById('app-view').classList.add('hidden');
      return;
    }
    const data = await resp.json();
    if (data.loggedIn && data.user) {
      state.user = data.user;
      state.boxId = data.boxId || '';
      state.cliRunning = !!data.cliRunning;
      state.guiRunning = !!data.guiRunning;
      state.envRunning = state.cliRunning || state.guiRunning;
      state.expiresIn = typeof data.expiresIn === 'number' ? data.expiresIn : 0;
      if (data.broadcast) state.broadcast = data.broadcast;

      document.getElementById('auth-view').classList.add('hidden');
      document.getElementById('app-view').classList.remove('hidden');

      document.getElementById('current-username').textContent = state.user.username;
      const roleTag = document.getElementById('current-role-tag');
      const isAdmin = state.user.role === 'admin';
      roleTag.textContent = isAdmin ? 'ADMIN' : 'USER';
      roleTag.className = `text-[10px] px-1 rounded ${isAdmin ? 'bg-zinc-900 text-white' : 'bg-zinc-700 text-white'}`;

      document.querySelectorAll('.admin-only').forEach(el => {
        el.classList.toggle('hidden', !isAdmin);
      });

      renderEnvStatus();
      renderBanner();
      startLeaseCountdown(state.expiresIn);
      switchTab('workspace');
    } else {
      document.getElementById('auth-view').classList.remove('hidden');
      document.getElementById('app-view').classList.add('hidden');
    }
  } catch (e) {
    console.error('initApp failed:', e);
    document.getElementById('auth-view').classList.remove('hidden');
    document.getElementById('app-view').classList.add('hidden');
  }
}

window.addEventListener('DOMContentLoaded', initApp);
