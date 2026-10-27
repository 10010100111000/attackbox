<script setup lang="ts">
import { ref, computed } from 'vue'
import { Search, RefreshCw, Users } from 'lucide-vue-next'
import { userStatusLabels, instanceStateLabels, type AdminUserList } from '~/shared/admin-users'
definePageMeta({ layout: 'admin' })
useHead({ title: '用户管理 — AttackBox' })
const searchInput = ref(''), search = ref(''), status = ref(''), role = ref(''), page = ref(1)
const query = computed(() => ({ search: search.value, status: status.value, role: role.value, page: page.value }))
const { data, pending, error, refresh } = await useLazyFetch<{ data: AdminUserList }>('/api/admin/users', { key: 'admin:users', query })
const result = computed(() => data.value?.data)
function filter() { page.value = 1; search.value = searchInput.value.trim() }
function date(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '从未登录' }
</script>
<template>
  <div class="users-page">
    <header class="page-heading"><div><h1>用户管理</h1><p>查看账号、学习记录与当前练习环境。</p></div><button :disabled="pending" @click="refresh()"><RefreshCw :size="14" aria-hidden="true" />刷新</button></header>
    <AdminDataLoading :pending="pending" :has-data="!!data" />
    <form class="filters" @submit.prevent="filter">
      <label class="search"><Search :size="16" aria-hidden="true" /><input v-model="searchInput" aria-label="搜索用户" placeholder="搜索邮箱或昵称" maxlength="100" /></label>
      <select v-model="status" aria-label="账号状态" @change="page = 1"><option value="">全部状态</option><option value="active">正常</option><option value="pending">待验证</option><option value="suspended">已停用</option></select>
      <select v-model="role" aria-label="账号角色" @change="page = 1"><option value="">全部角色</option><option value="user">用户</option><option value="admin">管理员</option></select>
      <button type="submit">搜索</button><span class="muted count">{{ result?.total ?? '—' }} 位用户</span>
    </form>
    <p v-if="error" class="notice" role="alert">无法读取用户，请检查网络或管理员登录状态。</p>
    <div v-else class="panel table-wrap" :aria-busy="pending">
      <table><thead><tr><th>用户</th><th>角色</th><th>状态</th><th>最近登录</th><th>当前环境</th><th class="actions">操作</th></tr></thead>
        <tbody><tr v-for="user in result?.items" :key="user.id"><td><NuxtLink :to="`/admin/users/${user.id}`" class="user-name">{{ user.displayName }}</NuxtLink><span class="secondary">{{ user.email }}</span></td><td>{{ user.role === 'admin' ? '管理员' : '用户' }}</td><td><span class="status" :class="user.status">{{ userStatusLabels[user.status] }}</span></td><td>{{ date(user.lastLoginAt) }}</td><td>{{ user.instance ? instanceStateLabels[user.instance.state] : '未运行' }}</td><td class="actions"><NuxtLink :to="`/admin/users/${user.id}`">查看</NuxtLink></td></tr></tbody>
      </table>
      <div v-if="!pending && !result?.items.length" class="empty"><Users :size="26" aria-hidden="true" /><p>没有符合条件的用户</p></div>
    </div>
    <footer v-if="result" class="pagination"><span class="muted">第 {{ page }} 页 · 每页 {{ result.pageSize }} 位</span><div><button :disabled="page <= 1 || pending" @click="page--">上一页</button><button :disabled="page * result.pageSize >= result.total || pending" @click="page++">下一页</button></div></footer>
  </div>
</template>
<style scoped src="~/assets/css/admin-users.css"></style>
