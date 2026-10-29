<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  BookOpen,
  Route,
  LayoutTemplate,
  Users,
  Server,
  Monitor,
  Settings,
  ExternalLink,
  LockKeyhole,
  Bell,
  MessageSquarePlus
} from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'

const route = useRoute()
const { currentUser, isInitialized, fetchMe } = useAuth()

const isChecking = ref(!isInitialized.value)

onMounted(async () => {
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

type AdminSectionKey =
  | 'runtime-policy'
  | 'learn-home'
  | 'notifications'
  | 'feedback'
  | 'challenges'
  | 'target-templates'
  | 'instances'
  | 'nodes'
  | 'references'
  | 'modules'
  | 'routes'
  | 'users'

const currentSection = computed<AdminSectionKey>(() => {
  if (route.path.startsWith('/admin/runtime-policy')) return 'runtime-policy'
  if (route.path.startsWith('/admin/learn-home')) return 'learn-home'
  if (route.path.startsWith('/admin/notifications')) return 'notifications'
  if (route.path.startsWith('/admin/feedback')) return 'feedback'
  if (route.path.startsWith('/admin/challenges')) return 'challenges'
  if (route.path.startsWith('/admin/target-templates')) return 'target-templates'
  if (route.path.startsWith('/admin/instances')) return 'instances'
  if (route.path.startsWith('/admin/nodes')) return 'nodes'
  if (route.path.startsWith('/admin/references')) return 'references'
  if (route.path.startsWith('/admin/modules')) return 'modules'
  if (route.path.startsWith('/admin/routes')) return 'routes'
  if (route.path.startsWith('/admin/users')) return 'users'
  return 'modules'
})

const sectionMeta: Record<
  AdminSectionKey,
  { groupLabel: string; groupHref: string; title: string }
> = {
  'runtime-policy': {
    groupLabel: '系统管理',
    groupHref: '/admin/runtime-policy',
    title: '运行策略'
  },
  'learn-home': { groupLabel: '内容管理', groupHref: '/admin/learn-home', title: '首页管理' },
  notifications: { groupLabel: '系统管理', groupHref: '/admin/notifications', title: '站内通知' },
  feedback: { groupLabel: '账户管理', groupHref: '/admin/feedback', title: '用户反馈' },
  challenges: { groupLabel: '内容管理', groupHref: '/admin/challenges', title: '挑战管理' },
  references: { groupLabel: '内容管理', groupHref: '/admin/references', title: '参考资料' },
  modules: { groupLabel: '内容管理', groupHref: '/admin/modules', title: '学习模块' },
  routes: { groupLabel: '内容管理', groupHref: '/admin/routes', title: '学习路线' },
  users: { groupLabel: '账户管理', groupHref: '/admin/users', title: '用户管理' },
  'target-templates': {
    groupLabel: '环境管理',
    groupHref: '/admin/target-templates',
    title: '靶场模板'
  },
  nodes: { groupLabel: '环境管理', groupHref: '/admin/nodes', title: '节点管理' },
  instances: { groupLabel: '环境管理', groupHref: '/admin/instances', title: '运行实例' }
}

const currentBreadcrumb = computed(() => sectionMeta[currentSection.value])
</script>

<template>
  <div class="admin-shell">
    <!-- 校验中 -->
    <div v-if="isChecking" class="admin-loading">正在验证管理员身份…</div>

    <!-- 非管理员拦截提示 -->
    <div v-else-if="!isAdmin" class="admin-denied">
      <div class="denied-card">
        <div class="denied-icon">
          <LockKeyhole :size="28" aria-hidden="true" />
        </div>
        <h2>需要管理员权限</h2>
        <p>当前登录账号不是管理员，无法访问管理后台。</p>
        <div class="denied-actions">
          <NuxtLink to="/learn" class="btn primary">返回学习平台</NuxtLink>
          <NuxtLink
            :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)"
            class="btn secondary"
          >
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
            to="/admin/challenges"
            class="admin-nav-link"
            :class="{ active: currentSection === 'challenges' }"
          >
            <Monitor :size="16" class="nav-icon" aria-hidden="true" />
            <span>挑战管理</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/modules"
            class="admin-nav-link"
            :class="{ active: currentSection === 'modules' }"
          >
            <BookOpen :size="16" class="nav-icon" aria-hidden="true" />
            <span>学习模块</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/references"
            class="admin-nav-link"
            :class="{ active: currentSection === 'references' }"
            ><BookOpen :size="16" class="nav-icon" aria-hidden="true" /><span
              >参考资料</span
            ></NuxtLink
          >
          <NuxtLink
            to="/admin/routes"
            class="admin-nav-link"
            :class="{ active: currentSection === 'routes' }"
          >
            <Route :size="16" class="nav-icon" aria-hidden="true" />
            <span>学习路线</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/learn-home"
            class="admin-nav-link"
            :class="{ active: currentSection === 'learn-home' }"
            ><LayoutTemplate :size="16" class="nav-icon" /><span>首页管理</span></NuxtLink
          >

          <div class="nav-group-label">账户管理</div>
          <NuxtLink
            to="/admin/users"
            class="admin-nav-link"
            :class="{ active: currentSection === 'users' }"
          >
            <Users :size="16" class="nav-icon" aria-hidden="true" />
            <span>用户管理</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/feedback"
            class="admin-nav-link"
            :class="{ active: currentSection === 'feedback' }"
            ><MessageSquarePlus :size="16" class="nav-icon" aria-hidden="true" /><span
              >用户反馈</span
            ></NuxtLink
          >

          <div class="nav-group-label">环境管理</div>
          <NuxtLink
            to="/admin/target-templates"
            class="admin-nav-link"
            :class="{ active: currentSection === 'target-templates' }"
          >
            <Monitor :size="16" class="nav-icon" aria-hidden="true" />
            <span>靶场模板</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/nodes"
            class="admin-nav-link"
            :class="{ active: currentSection === 'nodes' }"
          >
            <Server :size="16" class="nav-icon" aria-hidden="true" />
            <span>节点管理</span>
          </NuxtLink>
          <NuxtLink
            to="/admin/instances"
            class="admin-nav-link"
            :class="{ active: currentSection === 'instances' }"
          >
            <Monitor :size="16" class="nav-icon" aria-hidden="true" />
            <span>运行实例</span>
          </NuxtLink>

          <div class="nav-group-label">系统管理</div>
          <NuxtLink
            to="/admin/runtime-policy"
            class="admin-nav-link"
            :class="{ active: currentSection === 'runtime-policy' }"
            ><Settings :size="16" class="nav-icon" />运行策略</NuxtLink
          >
          <NuxtLink
            to="/admin/notifications"
            class="admin-nav-link"
            :class="{ active: currentSection === 'notifications' }"
            ><Bell :size="16" class="nav-icon" />站内通知</NuxtLink
          >
        </nav>

        <div class="sidebar-bottom">
          <NuxtLink to="/learn" class="bottom-link" target="_blank">
            <ExternalLink :size="14" aria-hidden="true" />
            <span>预览学习前台</span>
          </NuxtLink>
        </div>
      </aside>

      <!-- 右侧主体 -->
      <div class="admin-main-wrap">
        <header class="admin-topbar">
          <div class="topbar-crumb">
            <NuxtLink :to="currentBreadcrumb.groupHref">{{
              currentBreadcrumb.groupLabel
            }}</NuxtLink>
            <span>/</span>
            <span class="crumb-active">{{ currentBreadcrumb.title }}</span>
          </div>

          <div class="topbar-right">
            <UserMenu />
          </div>
        </header>

        <main class="admin-content">
          <div class="admin-page-container">
            <slot />
          </div>
        </main>
      </div>
    </template>
  </div>
</template>

<style scoped>
.admin-shell {
  --admin-page-width: 1400px;
  --admin-page-gutter: 32px;
  min-width: 0;
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
  display: flex;
  justify-content: center;
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

.admin-nav-link:hover {
  background: var(--soft);
  color: var(--ink);
}

.admin-nav-link.active {
  background: var(--mint);
  color: var(--green);
  font-weight: 600;
}

.nav-icon {
  flex-shrink: 0;
}

.sidebar-bottom {
  border-top: 1px solid var(--line);
  padding-top: 16px;
  margin-top: 12px;
}

.bottom-link {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--green);
  text-decoration: none;
  margin-bottom: 10px;
}

.bottom-link:hover {
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
  padding: 0 var(--admin-page-gutter);
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

.admin-content {
  flex: 1;
  padding: var(--admin-page-gutter);
  min-width: 0;
}

/* Page geometry belongs to the layout, not individual admin routes. */

.admin-page-container {
  width: 100%;
  max-width: var(--admin-page-width);
  min-width: 0;
}
</style>
