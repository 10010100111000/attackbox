<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BookOpen, Route, LayoutTemplate, Users, Server, Monitor, Settings, ExternalLink, LockKeyhole, Sun, Moon } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import { useTheme } from '~/composables/useTheme'

const route = useRoute()
const router = useRouter()
const { currentUser, isInitialized, fetchMe, logout } = useAuth()
const { currentTheme, setTheme, initTheme } = useTheme()

const isChecking = ref(!isInitialized.value)

onMounted(async () => {
  initTheme()
  if (!isInitialized.value) {
    try {
      await fetchMe()
    } finally {
      isChecking.value = false
    }
  }
})

const isAdmin = computed(() => {
  return currentUser.value?.role === 'admin'
})

const currentSection = computed(() => {
  if (route.path.startsWith('/admin/target-templates')) return 'target-templates'
  if (route.path.startsWith('/admin/instances')) return 'instances'
  if (route.path.startsWith('/admin/nodes')) return 'nodes'
  if (route.path.startsWith('/admin/modules')) return 'modules'
  if (route.path.startsWith('/admin/routes')) return 'routes'
  if (route.path.startsWith('/admin/users')) return 'users'
  return 'modules'
})

async function handleLogout() {
  await logout()
  router.push('/auth')
}
</script>

<template>
  <div class="admin-shell">
    <!-- 校验中 -->
    <div v-if="isChecking" class="admin-loading">
      正在验证管理员身份…
    </div>

    <!-- 非管理员拦截提示 -->
    <div v-else-if="!isAdmin" class="admin-denied">
      <div class="denied-card">
        <div class="denied-icon"><LockKeyhole :size="28" aria-hidden="true" /></div>
        <h2>需要管理员权限</h2>
        <p>当前登录账号不是管理员，无法访问管理后台。</p>
        <div class="denied-actions">
          <NuxtLink to="/learn" class="btn primary">返回学习平台</NuxtLink>
          <NuxtLink :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)" class="btn secondary">
            切换账号登录
          </NuxtLink>
        </div>
      </div>
    </div>

    <!-- 正常后台双栏结构 -->
    <template v-else>
      <!-- 左侧边栏 -->
      <aside class="admin-sidebar">
        <NuxtLink to="/admin/modules" class="admin-brand">
          <img src="/logo.png" alt="AttackBox Logo" class="brand-logo" width="28" height="28" />
          <span>AttackBox Admin</span>
        </NuxtLink>

        <nav class="admin-nav">
          <div class="nav-group-label">内容管理</div>
          <NuxtLink
            to="/admin/modules"
            class="admin-nav-link"
            :class="{ active: currentSection === 'modules' }"
          >
            <BookOpen :size="16" class="nav-icon" aria-hidden="true" />
            <span>学习模块</span>
          </NuxtLink>
          <NuxtLink to="/admin/routes" class="admin-nav-link" :class="{active:currentSection === 'routes'}"><Route :size="16" aria-hidden="true" /><span>学习路线</span></NuxtLink>
          <span class="admin-nav-link disabled" title="后续阶段接入">
            <LayoutTemplate :size="16" class="nav-icon" aria-hidden="true" />
            <span>首页管理</span>
            <span class="badge-plan">计划</span>
          </span>

          <div class="nav-group-label">账户管理</div>
          <NuxtLink to="/admin/users" class="admin-nav-link" :class="{ active: currentSection === 'users' }">
            <Users :size="16" class="nav-icon" aria-hidden="true" />
            <span>用户管理</span>
          </NuxtLink>

          <div class="nav-group-label">环境管理</div>
          <NuxtLink to="/admin/target-templates" class="admin-nav-link" :class="{active:currentSection === 'target-templates'}"><Monitor :size="16" aria-hidden="true"/><span>靶机模板</span></NuxtLink>
          <NuxtLink to="/admin/nodes" class="admin-nav-link" :class="{ active: currentSection === 'nodes' }">
            <Server :size="16" class="nav-icon" aria-hidden="true" />
            <span>节点管理</span>
          </NuxtLink>
          <NuxtLink to="/admin/instances" class="admin-nav-link" :class="{ active: currentSection === 'instances' }">
            <Monitor :size="16" class="nav-icon" aria-hidden="true" />
            <span>运行实例</span>
          </NuxtLink>

          <div class="nav-group-label">系统管理</div>
          <span class="admin-nav-link disabled" title="后续阶段接入">
            <Settings :size="16" class="nav-icon" aria-hidden="true" />
            <span>系统设置</span>
            <span class="badge-plan">计划</span>
          </span>
        </nav>

        <div class="sidebar-bottom">
          <NuxtLink to="/learn" class="bottom-link" target="_blank">
            <ExternalLink :size="14" aria-hidden="true" /><span>预览学习前台</span>
          </NuxtLink>
          <div class="admin-user-info">
            <span class="admin-email">{{ currentUser?.email }}</span>
            <button type="button" class="admin-logout-btn" @click="handleLogout">退出</button>
          </div>
        </div>
      </aside>

      <!-- 右侧主体 -->
      <div class="admin-main-wrap">
        <header class="admin-topbar">
          <div class="topbar-crumb">
            <NuxtLink :to="currentSection === 'target-templates' ? '/admin/target-templates' : currentSection === 'users' ? '/admin/users' : currentSection === 'nodes' || currentSection === 'instances' ? '/admin/nodes' : '/admin/modules'">{{ currentSection === 'target-templates' ? '环境管理' : currentSection === 'users' ? '账户管理' : currentSection === 'nodes' || currentSection === 'instances' ? '环境管理' : '内容管理' }}</NuxtLink>
            <span>/</span>
            <span class="crumb-active">{{ currentSection === 'target-templates' ? '靶机模板' : currentSection === 'routes' ? '学习路线' : currentSection === 'users' ? '用户管理' : currentSection === 'nodes' ? '节点管理' : currentSection === 'instances' ? '运行实例' : '学习模块' }}</span>
          </div>

          <div class="topbar-right">
            <div class="theme-switch-mini" role="group" aria-label="外观">
              <button
                type="button"
                :class="{ active: currentTheme === 'graphite' }"
                title="石墨"
                @click="setTheme('graphite')"
              >
                <Sun :size="15" aria-hidden="true" />
              </button>
              <button
                type="button"
                :class="{ active: currentTheme === 'midnight' }"
                title="深海"
                @click="setTheme('midnight')"
              >
                <Moon :size="15" aria-hidden="true" />
              </button>
            </div>
            <span class="admin-tag">管理员</span>
          </div>
        </header>

        <main class="admin-content">
          <slot />
        </main>
      </div>
    </template>
  </div>
</template>

<style scoped>
.admin-shell {
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  min-height: 100vh;
  background: var(--bg);
  color: var(--ink);
}

.admin-loading,
.admin-denied {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 32px;
}

.denied-card {
  max-width: 420px;
  width: 100%;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 36px 28px;
  text-align: center;
}

.denied-icon {
  font-size: 36px;
  margin-bottom: 12px;
}

.denied-card h2 {
  font-size: 18px;
  margin: 0 0 8px;
  color: var(--ink);
}

.denied-card p {
  font-size: 13px;
  color: var(--muted);
  margin: 0 0 20px;
}

.denied-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.denied-actions .btn {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
}

.denied-actions .btn.primary {
  background: var(--ink);
  color: var(--paper);
}

.denied-actions .btn.secondary {
  border: 1px solid var(--line);
  color: var(--ink);
}

/* 侧边栏 */
.admin-sidebar {
  background: var(--paper);
  border-right: 1px solid var(--line);
  padding: 24px 16px;
  position: sticky;
  top: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.admin-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 15px;
  letter-spacing: -0.3px;
  color: var(--ink);
  text-decoration: none;
  margin-bottom: 32px;
  padding-left: 6px;
}

.admin-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  overflow-y: auto;
}

.nav-group-label {
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
  text-transform: uppercase;
  margin: 18px 0 6px 8px;
}

.nav-group-label:first-child {
  margin-top: 0;
}

.admin-nav-link {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  font-size: 13px;
  color: var(--muted);
  text-decoration: none;
  transition: all 0.15s ease;
}

.admin-nav-link:hover:not(.disabled) {
  background: var(--soft);
  color: var(--ink);
}

.admin-nav-link.active {
  background: var(--mint);
  color: var(--green);
  font-weight: 600;
}

.admin-nav-link.disabled {
  opacity: 0.45;
  cursor: not-allowed;
  justify-content: space-between;
}

.admin-nav-link.disabled .nav-icon {
  margin-right: -4px;
}

.badge-plan {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--soft);
  color: var(--muted);
}

.nav-icon {
  font-size: 14px;
}

.sidebar-bottom {
  border-top: 1px solid var(--line);
  padding-top: 16px;
  margin-top: 12px;
}

.bottom-link {
  display: flex; align-items: center; gap: 6px;  font-size: 12px;
  color: var(--green);
  text-decoration: none;
  margin-bottom: 10px;
}

.bottom-link:hover {
  text-decoration: underline;
}

.admin-user-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: var(--muted);
}

.admin-email {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 120px;
}

.admin-logout-btn {
  background: transparent;
  border: none;
  color: var(--color-error);
  cursor: pointer;
  padding: 0;
  font-size: 11px;
}

.admin-logout-btn:hover {
  text-decoration: underline;
}

/* 顶栏 */
.admin-main-wrap {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.admin-topbar {
  height: 60px;
  border-bottom: 1px solid var(--line);
  padding: 0 32px;
  background: var(--paper);
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: sticky;
  top: 0;
  z-index: 20;
}

.topbar-crumb {
  font-size: 13px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px;
}

.topbar-crumb a {
  color: var(--muted);
  text-decoration: none;
}

.topbar-crumb a:hover {
  color: var(--green);
}

.crumb-active {
  color: var(--ink);
  font-weight: 500;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.theme-switch-mini {
  display: flex;
  background: var(--soft);
  border-radius: 6px;
  padding: 2px;
}

.theme-switch-mini button {
  background: transparent;
  border: none;
  padding: 3px 8px;
  font-size: 12px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--muted);
}

.theme-switch-mini button.active {
  background: var(--paper);
  color: var(--ink);
  font-weight: bold;
}

.admin-tag {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
  background: var(--mint);
  color: var(--green);
}

.admin-content {
  flex: 1;
  padding: 32px;
}
</style>
