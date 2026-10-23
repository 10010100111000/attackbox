<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '~/composables/useAuth'
import { useTheme } from '~/composables/useTheme'

const route = useRoute()
const router = useRouter()
const { currentUser, logout } = useAuth()
const { currentTheme, setTheme, initTheme } = useTheme()

const menuRef = ref<HTMLDetailsElement | null>(null)

onMounted(() => {
  initTheme()
})

const isLearnActive = computed(() => {
  return route.path.startsWith('/learn')
})

const isChallengesActive = computed(() => {
  return route.path.startsWith('/challenges')
})

const isDiscussActive = computed(() => {
  return route.path.startsWith('/discuss')
})

const userInitials = computed(() => {
  if (!currentUser.value) return '访'
  const name = currentUser.value.displayName || currentUser.value.email || ''
  return name.slice(0, 2).toUpperCase()
})

async function handleLogout() {
  if (menuRef.value) {
    menuRef.value.open = false
  }
  await logout()
  router.push('/learn')
}

function closeMenu() {
  if (menuRef.value) {
    menuRef.value.open = false
  }
}
</script>

<template>
  <header class="learning-header">
    <div class="header-left">
      <NuxtLink to="/learn" class="brand">
        <img src="/logo.png" alt="AttackBox Logo" class="brand-logo" width="31" height="31" />
        <span>AttackBox</span>
      </NuxtLink>
      <nav class="main-nav" aria-label="主导航">
        <NuxtLink to="/learn" :class="{ active: isLearnActive }">学习</NuxtLink>
        <NuxtLink to="/challenges" :class="{ active: isChallengesActive }">挑战</NuxtLink>
        <NuxtLink to="/discuss" :class="{ active: isDiscussActive }">讨论</NuxtLink>
      </nav>
    </div>

    <details ref="menuRef" class="user-menu" id="user-menu">
      <summary aria-label="打开用户菜单">
        <span class="avatar">{{ userInitials }}</span>
        <span class="user-chevron">⌄</span>
      </summary>
      <div class="user-panel">
        <div v-if="currentUser" class="user-info">
          <strong>{{ currentUser.displayName || currentUser.email }}</strong>
          <span>{{ currentUser.role === 'admin' ? '系统管理员' : '个人账户' }}</span>
        </div>
        <div v-else class="user-info">
          <strong>访客学习者</strong>
          <span>未登录</span>
        </div>

        <div v-if="currentUser" class="user-links">
          <NuxtLink
            v-if="currentUser.role === 'admin'"
            to="/admin/modules"
            class="user-link"
            @click="closeMenu"
          >
            <span>管理后台</span>
          </NuxtLink>
          <button
            type="button"
            class="user-link logout-btn"
            @click="handleLogout"
          >
            <span>退出登录</span>
          </button>
        </div>
        <div v-else class="user-links">
          <NuxtLink
            :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)"
            class="user-link"
            @click="closeMenu"
          >
            <span>登录 / 注册</span>
          </NuxtLink>
        </div>

        <div class="appearance">
          <span>外观</span>
          <div class="appearance-switch" role="group" aria-label="外观模式">
            <button
              type="button"
              :aria-pressed="currentTheme === 'graphite'"
              @click="setTheme('graphite')"
            >
              ☀ 石墨
            </button>
            <button
              type="button"
              :aria-pressed="currentTheme === 'midnight'"
              @click="setTheme('midnight')"
            >
              ☾ 深海
            </button>
          </div>
        </div>
      </div>
    </details>
  </header>
</template>

<style scoped>
.learning-header {
  height: 68px;
  border-bottom: 1px solid var(--line);
  background: var(--paper);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4%;
  position: sticky;
  top: 0;
  z-index: 50;
  color: var(--ink);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 36px;
  height: 100%;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 18px;
  letter-spacing: -0.4px;
  color: var(--ink);
  text-decoration: none;
}

.brand-logo {
  display: block;
  border-radius: 6px;
}

.main-nav {
  display: flex;
  align-items: center;
  gap: 28px;
  height: 100%;
}

.main-nav a {
  height: 100%;
  display: flex;
  align-items: center;
  border-bottom: 2px solid transparent;
  padding: 2px 2px 0;
  font-size: 14px;
  color: var(--muted);
  text-decoration: none;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.main-nav a:hover {
  color: var(--ink);
}

.main-nav a.active {
  color: var(--ink);
  border-bottom-color: var(--ink);
  font-weight: 600;
}

.user-menu {
  position: relative;
}

.user-menu summary {
  list-style: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 20px;
  transition: background 0.15s ease;
}

.user-menu summary:hover {
  background: var(--soft);
}

.user-menu summary::-webkit-details-marker {
  display: none;
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--mint);
  color: var(--green);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 12px;
  border: 1px solid var(--line);
}

.user-chevron {
  font-size: 12px;
  color: var(--muted);
}

.user-panel {
  position: absolute;
  right: 0;
  top: 44px;
  width: 240px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 10px;
  box-shadow: 0 12px 35px rgba(0, 0, 0, 0.25);
  z-index: 60;
}

.user-info {
  padding: 6px 8px 10px;
  border-bottom: 1px solid var(--line);
}

.user-info strong {
  display: block;
  font-size: 14px;
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-info span {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-top: 2px;
}

.user-links {
  padding: 6px 0;
}

.user-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px;
  font-size: 13px;
  color: var(--ink);
  text-decoration: none;
  border-radius: 6px;
  background: transparent;
  border: none;
  cursor: pointer;
  width: 100%;
  text-align: left;
  transition: background 0.15s ease;
}

.user-link:hover {
  background: var(--hover);
}

.user-link span {
  color: var(--muted);
  font-size: 12px;
}

.logout-btn {
  color: var(--color-error);
}

.logout-btn:hover {
  background: rgba(229, 115, 115, 0.1);
}

.appearance {
  padding: 10px 8px 4px;
  border-top: 1px solid var(--line);
}

.appearance > span {
  display: block;
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 8px;
}

.appearance-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  border-radius: 8px;
  padding: 3px;
  background: var(--soft);
}

.appearance-switch button {
  background: transparent;
  color: var(--muted);
  padding: 6px 4px;
  font-size: 12px;
  border-radius: 6px;
  border: none;
  cursor: pointer;
  transition: all 0.15s ease;
}

.appearance-switch button[aria-pressed='true'] {
  background: var(--paper);
  color: var(--ink);
  font-weight: 600;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.12);
}

@media (max-width: 640px) {
  .learning-header {
    padding: 0 16px;
  }
  .header-left {
    gap: 16px;
  }
  .main-nav {
    gap: 16px;
  }
}
</style>
