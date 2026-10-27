<script setup lang="ts">
import { Bell, BookOpen, Flag, Settings2, UserRound } from 'lucide-vue-next'

definePageMeta({ layout: 'learning' })
const { currentUser } = useAuth()
const route = useRoute()
const sections = [
  { id: 'learning', label: '学习记录', icon: BookOpen },
  { id: 'challenges', label: '挑战记录', icon: Flag },
  { id: 'profile', label: '账户资料', icon: Settings2 },
] as const
const section = computed(
  () => sections.find((item) => item.id === route.query.tab) || sections[0],
)
useHead({
  title: computed(() => `${section.value.label} · 个人中心 — AttackBox`),
})
</script>

<template>
  <div class="learning-shell learning-shell--sidebar personal-center">
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
          <span class="personal-eyebrow">个人中心</span>
          <h2>{{ currentUser.displayName }}</h2>
          <span class="personal-role">
            {{ currentUser.role === 'admin' ? '管理员' : '学习者' }}
          </span>
        </div>
        <nav class="personal-navigation" aria-label="个人中心">
          <NuxtLink
            v-for="item in sections"
            :key="item.id"
            :to="
              item.id === 'learning'
                ? '/me'
                : { path: '/me', query: { tab: item.id } }
            "
            :aria-current="section.id === item.id ? 'page' : undefined"
          >
            <component :is="item.icon" :size="17" aria-hidden="true" />
            {{ item.label }}
          </NuxtLink>
        </nav>
        <NuxtLink to="/notifications" class="personal-notifications">
          <Bell :size="16" aria-hidden="true" />
          通知中心
        </NuxtLink>
      </aside>
      <div :key="currentUser.id" class="personal-content">
        <AccountLearning v-if="section.id === 'learning'" />
        <AccountChallenges v-else-if="section.id === 'challenges'" />
        <!-- Preserve an unsaved nickname when moving between these sections. -->
        <AccountProfile v-show="section.id === 'profile'" />
      </div>
    </template>
    <section v-else class="personal-guest">
      <UserRound :size="30" aria-hidden="true" />
      <h1>个人中心</h1>
      <p>登录后查看学习与挑战记录，继续上次的进度。</p>
      <NuxtLink to="/auth" class="personal-button">登录 / 注册</NuxtLink>
    </section>
  </div>
</template>

<style src="~/assets/css/personal-center.css"></style>
