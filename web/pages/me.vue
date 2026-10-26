<script setup lang="ts">
import { Bell, BookOpen } from 'lucide-vue-next'
definePageMeta({ layout: 'learning' })
useHead({ title: '个人中心 — AttackBox' })
const { currentUser, fetchMe } = useAuth(),
  name = ref(currentUser.value?.displayName || ''),
  busy = ref(false),
  message = ref('')
watch(
  () => currentUser.value?.id,
  () => {
    name.value = currentUser.value?.displayName || ''
    message.value = ''
  },
)
async function save() {
  busy.value = true
  message.value = ''
  try {
    await $fetch('/api/me/profile', {
      method: 'PUT',
      body: { displayName: name.value },
      retry: false,
    })
    await fetchMe()
    message.value = '资料已保存'
  } catch (e: any) {
    message.value = e?.data?.message || '保存失败'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="profile-page">
    <h1>个人中心</h1>
    <p class="intro">管理你的账户资料</p>
    <template v-if="currentUser">
      <section class="profile-card">
        <div class="profile-avatar">
          {{ currentUser.displayName.slice(0, 2).toUpperCase() }}
        </div>
        <div>
          <h2>{{ currentUser.displayName }}</h2>
          <p>{{ currentUser.email }}</p>
        </div>
      </section>
      <form @submit.prevent="save">
        <h2>基本资料</h2>
        <label>
          昵称
          <input
            v-model="name"
            maxlength="64"
            required
            autocomplete="nickname"
          />
        </label>
        <label>
          邮箱
          <span class="email-value">{{ currentUser.email }}</span>
        </label>
        <p v-if="message" role="status">{{ message }}</p>
        <button :disabled="busy || !name.trim()">
          {{ busy ? '正在保存…' : '保存资料' }}
        </button>
      </form>
      <nav class="profile-links">
        <NuxtLink to="/notifications">
          <Bell :size="17" />
          通知中心
        </NuxtLink>
        <NuxtLink to="/learn">
          <BookOpen :size="17" />
          继续学习
        </NuxtLink>
      </nav>
    </template>
    <p v-else>
      登录后查看个人中心。
      <NuxtLink to="/auth">登录</NuxtLink>
    </p>
  </div>
</template>
<style scoped>
.profile-page {
  width: min(760px, calc(100% - 48px));
  margin: 46px auto;
}
h1 {
  font-size: 26px;
  margin: 0;
}
.intro,
.profile-card p {
  font-size: 13px;
  color: var(--muted);
}
.profile-card,
form {
  margin-top: 24px;
  border: 1px solid var(--line);
  background: var(--paper);
  border-radius: 14px;
  padding: 28px;
}
.profile-card {
  display: flex;
  align-items: center;
  gap: 18px;
}
.profile-avatar {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 50%;
  background: var(--soft);
  color: var(--green);
  font-weight: 650;
}
.profile-card h2 {
  margin: 0;
  font-size: 20px;
}
.profile-card p {
  margin: 8px 0 0;
  overflow-wrap: anywhere;
}
form h2 {
  font-size: 16px;
  margin: 0 0 24px;
}
label {
  display: block;
  font-size: 13px;
  margin: 20px 0;
}
input {
  display: block;
  width: min(100%, 340px);
  box-sizing: border-box;
  margin-top: 9px;
  padding: 11px 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--bg);
  color: var(--ink);
  font: inherit;
}
.email-value {
  display: block;
  margin-top: 10px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
button {
  border: 0;
  border-radius: 8px;
  background: var(--ink);
  color: var(--paper);
  padding: 10px 18px;
  cursor: pointer;
}
button:disabled {
  opacity: 0.5;
}
.profile-links {
  display: flex;
  gap: 24px;
  margin-top: 26px;
}
.profile-links a {
  display: flex;
  gap: 9px;
  align-items: center;
  text-decoration: none;
  color: var(--muted);
  font-size: 13px;
}
</style>
