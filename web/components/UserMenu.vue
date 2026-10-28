<script setup lang="ts">
import {
  PopoverRoot,
  PopoverTrigger,
  PopoverPortal,
  PopoverContent,
} from 'reka-ui'
import { Bell, UserRound, Shield, LogOut, MessageSquarePlus } from 'lucide-vue-next'
const { currentUser, logout } = useAuth(),
  { initTheme } = useTheme(),
  { unread, refreshUnread } = useNotifications()
const open = ref(false),
  feedbackOpen = ref(false),
  menuTrigger = ref<{ $el: HTMLElement } | null>(null),
  leaving = ref(false),
  error = ref(''),
  route = useRoute()
onMounted(() => {
  initTheme()
  void refreshUnread()
})
watch(
  () => currentUser.value?.id,
  () => {
    open.value = false
    feedbackOpen.value = false
    void refreshUnread()
  },
)
watch(open, (v) => {
  if (v) void refreshUnread()
})
watch(
  () => route.fullPath,
  () => { open.value = false; feedbackOpen.value = false },
)
async function composeFeedback() {
  open.value = false
  // Let the popover release its focus trap before opening the dialog.
  await nextTick()
  feedbackOpen.value = true
}
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
      ref="menuTrigger"
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
          <button type="button" @click="composeFeedback">
            <MessageSquarePlus :size="16" />
            问题反馈
          </button>
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
        <div class="account-actions">
          <button
            v-if="currentUser"
            class="account-exit"
            :disabled="leaving"
            @click="exit"
          >
            <LogOut :size="15" />
            <UiBusyLabel :busy="leaving">退出登录</UiBusyLabel>
          </button>
          <UiThemeToggle />
        </div>
        <p v-if="error" role="alert">{{ error }}</p>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
  <FeedbackComposer v-model:open="feedbackOpen" @restore-focus="menuTrigger?.$el.focus()" />
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
.account-panel nav button,
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
.account-panel nav button:hover,
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
.account-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding: 6px 4px 4px;
  border-top: 1px solid var(--line);
  margin-top: 6px;
}
.account-exit {
  width: auto;
  margin-right: auto;
  color: var(--error-ink);
}
.account-panel p {
  padding: 0 12px;
  color: var(--error-ink);
}
</style>
