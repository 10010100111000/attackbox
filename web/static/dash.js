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
  el.className = 'bg-zinc-900 text-white px-3 py-2 rounded-lg shadow text-xs font-medium';
  el.textContent = msg;
  container.appendChild(el);
  setTimeout(() => el.remove(), 2500);
}

// ==================== 登录 / 注册 Tab 切换与提交 ====================

let authMode = 'login';

function switchAuthTab(mode) {
  authMode = mode;
  const isLogin = mode === 'login';
  document.getElementById('tab-login').className = isLogin
    ? 'py-1.5 text-xs font-medium rounded-md bg-white text-zinc-900 shadow-sm'
    : 'py-1.5 text-xs font-medium rounded-md text-zinc-500';
  document.getElementById('tab-register').className = !isLogin
    ? 'py-1.5 text-xs font-medium rounded-md bg-white text-zinc-900 shadow-sm'
    : 'py-1.5 text-xs font-medium rounded-md text-zinc-500';
  document.getElementById('auth-submit-btn').textContent = isLogin ? '进入平台' : '注册并登录';
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
  showToast('正在新标签页开启 Web 虚拟终端空间...');
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
  showToast('正在新标签页开启 GUI 远程桌面...');
}

// ==================== 沙箱状态维护与彻底销毁 ====================

function renderEnvStatus() {
  const isRunning = !!(state.envRunning || state.cliRunning || state.guiRunning);
  state.envRunning = isRunning;

  const badge = document.getElementById('env-status-badge');
  const badgeDot = document.getElementById('env-status-dot');
  const badgeText = document.getElementById('env-status-text');
  const btnDestroy = document.getElementById('btn-destroy-env');

  if (!badge) return;

  if (isRunning) {
    badge.className = 'px-2 py-0.5 rounded-full text-[11px] font-medium bg-emerald-50 text-emerald-700 border border-emerald-200 flex items-center';
    badgeDot.className = 'w-1.5 h-1.5 rounded-full bg-emerald-500 mr-1.5 animate-pulse';
    badgeText.textContent = '运行中 (已就绪)';
    if (btnDestroy) btnDestroy.classList.remove('hidden');
  } else {
    badge.className = 'px-2 py-0.5 rounded-full text-[11px] font-medium bg-zinc-100 text-zinc-500 border border-zinc-200 flex items-center';
    badgeDot.className = 'w-1.5 h-1.5 rounded-full bg-zinc-400 mr-1.5';
    badgeText.textContent = '未启动';
    if (btnDestroy) btnDestroy.classList.add('hidden');
  }
}

async function destroyEnv() {
  if (!confirm('确定要彻底释放当前渗透沙箱吗？未保存的数据将被清空，所有正在运行的命令会话与桌面将被终止。')) return;
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
  state.activeTab = tab;
  ['workspace', 'admin-vps', 'admin-users', 'admin-broadcast'].forEach(id => {
    const el = document.getElementById(`view-${id}`);
    if (el) el.classList.toggle('hidden', id !== tab);
  });
  document.querySelectorAll('.nav-tab').forEach(btn => {
    const active = btn.getAttribute('data-tab') === tab;
    btn.className = `nav-tab ${btn.classList.contains('admin-only') ? 'admin-only' : ''} px-2.5 py-1.5 rounded-md text-xs font-medium flex items-center gap-1 ${
      active ? 'bg-zinc-900 text-white' : 'text-zinc-600 hover:bg-zinc-100'
    }`;
    if (state.user && state.user.role !== 'admin' && btn.classList.contains('admin-only')) {
      btn.classList.add('hidden');
    }
  });
  if (tab === 'admin-vps') fetchMetrics();
  if (tab === 'admin-users') fetchUsers();
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
  tbody.innerHTML = state.users.map(u => `
    <tr>
      <td class="py-2.5 px-4 font-medium text-zinc-900">${u.username}</td>
      <td class="py-2.5 px-4">
        <span class="px-2 py-0.5 rounded text-[10px] font-medium ${u.role === 'admin' ? 'bg-zinc-900 text-white' : 'bg-zinc-100 text-zinc-700'}">
          ${u.role === 'admin' ? '管理员' : '普通用户'}
        </span>
      </td>
      <td class="py-2.5 px-4">${u.active ? '<span class="text-emerald-600 font-medium">正常</span>' : '<span class="text-zinc-400">已禁用</span>'}</td>
      <td class="py-2.5 px-4 text-zinc-400 font-mono text-[11px]">${u.created_at ? u.created_at.slice(0, 10) : '-'}</td>
      <td class="py-2.5 px-4 text-right space-x-1.5">
        <button onclick="openResetPassModal('${u.username}')" class="px-2 py-1 rounded border border-zinc-200 hover:bg-zinc-100 text-zinc-600">重置密码</button>
        <button onclick="toggleUserActive('${u.username}')" class="px-2 py-1 rounded border border-zinc-200 hover:bg-zinc-100 text-zinc-600">
          ${u.active ? '禁用' : '启用'}
        </button>
        <button onclick="deleteUser('${u.username}')" class="px-2 py-1 rounded border border-zinc-200 hover:bg-red-50 text-red-600">删除</button>
      </td>
    </tr>
  `).join('');
}

function openAddUserModal() {
  document.getElementById('modal-title').textContent = '手动添加 Web 用户';
  document.getElementById('modal-body').innerHTML = `
    <form onsubmit="submitAddUser(event)" class="space-y-3.5 text-xs">
      <div>
        <label class="block font-medium text-zinc-700 mb-1">用户名</label>
        <input id="new-user-name" required placeholder="输入字母数字用户名" class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div>
        <label class="block font-medium text-zinc-700 mb-1">初始密码</label>
        <input id="new-user-pass" type="password" required placeholder="至少 6 位密码" class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div>
        <label class="block font-medium text-zinc-700 mb-1">角色权限</label>
        <select id="new-user-role" class="w-full px-3 py-2 border border-zinc-200 rounded-lg">
          <option value="user">普通用户 (User)</option>
          <option value="admin">管理员 (Admin)</option>
        </select>
      </div>
      <div class="pt-2 flex justify-end gap-2">
        <button type="button" onclick="closeModal()" class="px-3 py-1.5 rounded-lg shadcn-btn-outline">取消</button>
        <button type="submit" class="px-3.5 py-1.5 rounded-lg shadcn-btn-primary">确认添加</button>
      </div>
    </form>
  `;
  document.getElementById('modal-overlay').classList.remove('hidden');
}

async function submitAddUser(e) {
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
      showToast(data.error || '创建用户失败');
      return;
    }
    closeModal();
    fetchUsers();
    showToast('已成功创建用户: ' + username);
  } catch (err) {
    showToast('请求异常: ' + err.message);
  }
}

function openResetPassModal(username) {
  document.getElementById('modal-title').textContent = `重置用户【${username}】密码`;
  document.getElementById('modal-body').innerHTML = `
    <form onsubmit="submitResetPass(event, '${username}')" class="space-y-3.5 text-xs">
      <div>
        <label class="block font-medium text-zinc-700 mb-1">新密码</label>
        <input id="reset-user-pass" type="password" required placeholder="输入新密码" class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900" />
      </div>
      <div class="pt-2 flex justify-end gap-2">
        <button type="button" onclick="closeModal()" class="px-3 py-1.5 rounded-lg shadcn-btn-outline">取消</button>
        <button type="submit" class="px-3.5 py-1.5 rounded-lg shadcn-btn-primary">保存密码</button>
      </div>
    </form>
  `;
  document.getElementById('modal-overlay').classList.remove('hidden');
}

async function submitResetPass(e, username) {
  e.preventDefault();
  const newPassword = document.getElementById('reset-user-pass').value;
  try {
    const resp = await fetch('/api/admin/users/reset-pass', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password: newPassword })
    });
    const data = await resp.json();
    if (!resp.ok || !data.success) {
      showToast(data.error || '重置失败');
      return;
    }
    closeModal();
    showToast(`已重置【${username}】的密码`);
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
  const input = document.createElement('input');
  input.value = ip;
  document.body.appendChild(input);
  input.select();
  document.execCommand('copy');
  input.remove();
  showToast('已复制 IP: ' + ip);
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
      if (data.broadcast) state.broadcast = data.broadcast;

      document.getElementById('auth-view').classList.add('hidden');
      document.getElementById('app-view').classList.remove('hidden');

      document.getElementById('current-username').textContent = state.user.username;
      const slotEl = document.getElementById('box-slot-id');
      if (slotEl) {
        slotEl.textContent = state.boxId ? `${state.boxId} (独占隔离)` : '自动调度';
      }
      const roleTag = document.getElementById('current-role-tag');
      const isAdmin = state.user.role === 'admin';
      roleTag.textContent = isAdmin ? 'ADMIN' : 'USER';
      roleTag.className = `text-[10px] px-1 rounded ${isAdmin ? 'bg-zinc-900 text-white' : 'bg-emerald-600 text-white'}`;

      document.querySelectorAll('.admin-only').forEach(el => {
        el.classList.toggle('hidden', !isAdmin);
      });

      renderEnvStatus();
      renderBanner();
      switchTab('workspace');
      updateActiveWindowsPills();
    } else {
      document.getElementById('auth-view').classList.remove('hidden');
      document.getElementById('app-view').classList.add('hidden');
    }
  } catch (e) {
    document.getElementById('auth-view').classList.remove('hidden');
    document.getElementById('app-view').classList.add('hidden');
  }
}

window.addEventListener('DOMContentLoaded', initApp);
