<script setup lang="ts">
import {
  PopoverRoot,
  PopoverTrigger,
  PopoverPortal,
  PopoverContent,
} from 'reka-ui'
import { Bell, UserRound, Shield, LogOut, Sun, Moon } from 'lucide-vue-next'
const { currentUser, logout } = useAuth(),
  { currentTheme, setTheme, initTheme } = useTheme(),
  { unread, refreshUnread } = useNotifications()
const open = ref(false),
  leaving = ref(false),
  error = ref(''),
  themeId = useId(),
  route = useRoute()
onMounted(() => {
  initTheme()
  void refreshUnread()
})
watch(
  () => currentUser.value?.id,
  () => {
    open.value = false
    void refreshUnread()
  },
)
watch(open, (v) => {
  if (v) void refreshUnread()
})
watch(
  () => route.fullPath,
  () => (open.value = false),
)
async function exit() {
  leaving.value = true
  error.value = ''
  try {
    await logout()
    open.value = false
    await navigateTo('/learn')
  } catch {
    error.value = '退出未确认，请重试'
  } finally {
    leaving.value = false
  }
}
</script>
<template>
  <PopoverRoot v-model:open="open" :modal="true">
    <PopoverTrigger
      class="account-trigger"
      :aria-label="unread ? `用户菜单，${unread} 条未读通知` : '用户菜单'"
    >
      <span>
        {{
          currentUser
            ? (currentUser.displayName || currentUser.email)
                .slice(0, 2)
                .toUpperCase()
            : '访'
        }}
      </span>
      <i v-if="unread" class="account-dot" />
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent
        class="account-panel"
        align="end"
        :side-offset="10"
        aria-label="用户菜单"
      >
        <div class="account-identity">
          <strong>{{ currentUser?.displayName || '访客' }}</strong>
          <span>{{ currentUser?.email || '登录后继续学习' }}</span>
        </div>
        <nav v-if="currentUser" aria-label="账户导航" @click="open = false">
          <NuxtLink to="/me">
            <UserRound :size="16" />
            个人中心
          </NuxtLink>
          <NuxtLink to="/notifications">
            <Bell :size="16" />
            通知
            <span v-if="unread" class="account-count">
              {{ unread > 99 ? '99+' : unread }}
            </span>
          </NuxtLink>
          <NuxtLink v-if="currentUser.role === 'admin'" to="/admin/modules">
            <Shield :size="16" />
            后台管理
          </NuxtLink>
        </nav>
        <nav v-else @click="open = false">
          <NuxtLink to="/auth">
            <UserRound :size="16" />
            登录 / 注册
          </NuxtLink>
        </nav>
        <div class="account-appearance">
          <span>外观</span>
          <div class="appearance-control" role="radiogroup" aria-label="外观">
            <i
              class="appearance-thumb"
              :class="{ dark: currentTheme === 'midnight' }"
            />
            <label>
              <input
                type="radio"
                :name="themeId"
                value="graphite"
                :checked="currentTheme === 'graphite'"
                @change="setTheme('graphite')"
              />
              <span>
                <Sun :size="13" />
                石墨
              </span>
            </label>
            <label>
              <input
                type="radio"
                :name="themeId"
                value="midnight"
                :checked="currentTheme === 'midnight'"
                @change="setTheme('midnight')"
              />
              <span>
                <Moon :size="13" />
                深海
              </span>
            </label>
          </div>
        </div>
        <button
          v-if="currentUser"
          class="account-exit"
          :disabled="leaving"
          @click="exit"
        >
          <LogOut :size="15" />
          <UiBusyLabel :busy="leaving">退出登录</UiBusyLabel>
        </button>
        <p v-if="error" role="alert">{{ error }}</p>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
<style scoped>
.account-trigger {
  position: relative;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border: 1px solid var(--avatar-border);
  border-radius: 50%;
  background: var(--avatar-bg);
  color: var(--avatar-fg);
  box-shadow: var(--avatar-shadow);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.3px;
  cursor: pointer;
  flex-shrink: 0;
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.account-trigger:hover {
  border-color: var(--brand);
  transform: translateY(-1px);
}
.account-trigger:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
.account-dot {
  position: absolute;
  right: 0;
  top: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--green);
  border: 2px solid var(--paper);
}
:global(.account-panel) {
  z-index: 200;
  width: 260px;
  max-width: calc(100vw - 32px);
  padding: 8px;
  border: 1px solid var(--popover-border);
  border-radius: 16px;
  background: var(--popover-bg);
  color: var(--ink);
  box-shadow: var(--surface-highlight), 0 18px 48px rgba(0, 0, 0, 0.48);
  font-size: 13px;
}
.account-identity {
  display: grid;
  gap: 5px;
  padding: 12px 12px 16px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 6px;
}
.account-identity strong {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.account-identity > span {
  font-size: 11px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.account-panel nav a,
.account-exit {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
  border: 0;
  background: transparent;
  text-decoration: none;
  padding: 11px 12px;
  color: inherit;
  border-radius: 9px;
  font: inherit;
  cursor: pointer;
}
.account-panel nav a:hover,
.account-exit:hover {
  background: var(--soft);
}
.account-count {
  margin-left: auto;
  font-size: 11px;
  background: var(--soft);
  padding: 1px 7px;
  border-radius: 20px;
}
.account-appearance {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  border-top: 1px solid var(--line);
  margin-top: 6px;
  color: var(--muted);
  font-size: 12px;
}
.appearance-control {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  padding: 3px;
  background: var(--soft);
  border-radius: 10px;
  width: 148px;
}
.appearance-thumb {
  position: absolute;
  left: 3px;
  top: 3px;
  bottom: 3px;
  width: calc(50% - 3px);
  border-radius: 7px;
  background: var(--paper);
  box-shadow: 0 1px 4px #0002;
  transition: transform 0.18s ease;
}
.appearance-thumb.dark {
  transform: translateX(100%);
}
.appearance-control label {
  position: relative;
  cursor: pointer;
}
.appearance-control input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  opacity: 0;
  z-index: 1;
  cursor: pointer;
}
.appearance-control label span {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 6px 0;
}
.appearance-control input:checked + span {
  color: var(--ink);
}
.appearance-control input:focus-visible + span {
  outline: 2px solid var(--green);
  border-radius: 7px;
}
.account-exit {
  color: var(--error-ink);
  border-top: 1px solid var(--line);
  margin-top: 2px;
  border-radius: 0 0 9px 9px;
}
.account-panel p {
  padding: 0 12px;
  color: var(--error-ink);
}
@media (prefers-reduced-motion: reduce) {
  .appearance-thumb {
    transition: none;
  }
}
</style>
