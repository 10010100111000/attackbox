<script setup lang="ts">
import { Bell, BookOpen, Flag, Settings2 } from 'lucide-vue-next'

const props = defineProps<{
  section: 'learning' | 'challenges' | 'profile' | 'notifications'
}>()

const { currentUser } = useAuth()

const sections = [
  { id: 'learning', label: '学习记录', icon: BookOpen, to: '/me' },
  { id: 'challenges', label: '挑战记录', icon: Flag, to: '/me?tab=challenges' },
  { id: 'profile', label: '账户资料', icon: Settings2, to: '/me?tab=profile' },
  { id: 'notifications', label: '通知', icon: Bell, to: '/notifications' }
]
</script>

<template>
  <div class="learning-frame learning-shell personal-center" :data-section="props.section">
    <template v-if="currentUser">
      <aside class="personal-sidebar">
        <div class="personal-identity">
          <span class="personal-avatar" aria-hidden="true">
            {{ (currentUser.displayName || currentUser.email).slice(0, 2).toUpperCase() }}
          </span>
          <h2>{{ currentUser.displayName }}</h2>
          <span v-if="currentUser.role === 'admin'" class="personal-role"> 管理员 </span>
        </div>
        <nav class="personal-navigation" aria-label="个人中心">
          <NuxtLink
            v-for="item in sections"
            :key="item.id"
            :to="item.to"
            :data-section="item.id"
            :aria-current="section === item.id ? 'page' : undefined"
          >
            <component :is="item.icon" :size="16" aria-hidden="true" />
            {{ item.label }}
          </NuxtLink>
        </nav>
      </aside>
      <div :key="currentUser.id" class="personal-content">
        <slot />
      </div>
    </template>
    <section v-else class="personal-guest">
      <h1>{{ sections.find(s => s.id === section)?.label }}</h1>
      <NuxtLink to="/auth" class="personal-button">登录 / 注册</NuxtLink>
    </section>
  </div>
</template>

<style src="~/assets/css/personal-center.css"></style>

<style scoped>
.personal-sidebar {
  position: sticky;
  top: 92px;
  min-width: 0;
}

.personal-identity {
  padding: 4px 0 24px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 16px;
}

.personal-avatar:hover {
  transform: translateY(-1px);
  border-color: var(--brand-border);
}

.personal-identity h2 {
  margin: 0 0 6px;
  font-size: 17px;
  font-weight: 650;
  line-height: 1.4;
  overflow-wrap: anywhere;
  letter-spacing: -0.3px;
}

.personal-role {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--soft);
  border: 1px solid var(--line);
  color: var(--muted);
  font-size: 11px;
  font-weight: 550;
}

.personal-navigation {
  display: grid;
  gap: 4px;
}

.personal-navigation a {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 11px;
  border-radius: 9px;
  border: 1px solid transparent;
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  transition:
    background 0.16s ease,
    color 0.16s ease,
    border-color 0.16s ease,
    transform 0.16s ease;
}

.personal-navigation a > svg {
  color: var(--muted);
  flex-shrink: 0;
  transition:
    color 0.16s ease,
    transform 0.16s ease;
}

.personal-navigation a:hover {
  background: var(--soft);
  color: var(--ink);
}

.personal-navigation a:hover > svg {
  color: var(--ink);
  transform: translateX(1px);
}

.personal-navigation a[aria-current='page'] {
  background: var(--soft);
  color: var(--ink);
  border-color: var(--line);
  box-shadow: var(--surface-highlight);
  font-weight: 600;
}

.personal-navigation a[aria-current='page'] > svg {
  color: var(--brand);
}

.personal-content {
  min-width: 0;
  animation: surface-rise 0.28s cubic-bezier(0.22, 1, 0.36, 1) both;
}

.personal-guest {
  padding: 64px 28px;
  text-align: center;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: var(--paper);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  grid-column: 1 / -1;
  width: min(600px, 100%);
  box-sizing: border-box;
  margin: 24px auto;
}

.personal-guest > svg {
  margin: 0 auto 18px;
  color: var(--muted);
}

.personal-guest h1 {
  margin: 0 0 10px;
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.4px;
}

.personal-guest p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.8;
}

.personal-guest .personal-button {
  margin-top: 24px;
}

@media (max-width: 760px) {
  .personal-sidebar {
    position: static;
  }

  .personal-identity {
    display: grid;
    grid-template-columns: 48px minmax(0, 1fr) auto;
    align-items: center;
    column-gap: 12px;
    padding: 0 2px 18px;
    margin-bottom: 14px;
  }

  .personal-avatar {
    grid-row: 1 / 3;
    width: 46px;
    height: 46px;
    margin: 0;
    font-size: 16px;
    border-radius: 14px;
  }

  .personal-identity h2 {
    grid-column: 2;
    grid-row: 1 / 3;
    margin: 0;
    font-size: 16px;
  }

  .personal-role {
    grid-column: 3;
    grid-row: 1 / 3;
    font-size: 11px;
  }

  .personal-navigation {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    padding: 4px;
    border: 1px solid var(--line);
    background: var(--soft);
    border-radius: 11px;
    gap: 4px;
  }

  .personal-navigation a {
    flex-direction: column;
    justify-content: center;
    gap: 6px;
    padding: 10px 4px;
    font-size: 12px;
  }

  .personal-navigation svg {
    width: 15px;
  }
}
</style>
