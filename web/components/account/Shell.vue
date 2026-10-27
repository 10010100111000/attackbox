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
  { id: 'notifications', label: '通知', icon: Bell, to: '/notifications' },
]
</script>

<template>
  <div
    class="learning-shell learning-shell--sidebar personal-center"
    :data-section="props.section"
  >
    <template v-if="currentUser">
      <aside class="personal-sidebar">
        <div class="personal-identity">
          <span class="personal-avatar" aria-hidden="true">
            {{
              (currentUser.displayName || currentUser.email)
                .slice(0, 2)
                .toUpperCase()
            }}
          </span>
          <h2>{{ currentUser.displayName }}</h2>
          <span v-if="currentUser.role === 'admin'" class="personal-role">
            管理员
          </span>
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
      <h1>{{ sections.find((s) => s.id === section)?.label }}</h1>
      <NuxtLink to="/auth" class="personal-button">登录 / 注册</NuxtLink>
    </section>
  </div>
</template>

<style src="~/assets/css/personal-center.css"></style>
