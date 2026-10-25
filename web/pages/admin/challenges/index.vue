<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, Search } from 'lucide-vue-next'
import type { ChallengeSummary } from '~/shared/challenges'

definePageMeta({ layout: 'admin' })
useHead({ title: '挑战管理 — AttackBox' })

const { data, pending, error, refresh } = await useLazyFetch<{ data: ChallengeSummary[] }>('/api/admin/challenges', {
  key: 'admin:challenges',
})

const query = ref('')
const notice = ref('')

const rows = computed(() =>
  data.value?.data.filter(c =>
    `${c.title} ${c.category}`.toLowerCase().includes(query.value.toLowerCase())
  ) || []
)

async function remove(c: ChallengeSummary) {
  if (!confirm(`删除“${c.title}”及阶段完成记录？`)) return
  try {
    await $fetch(`/api/admin/challenges/${c.id}`, {
      method: 'DELETE',
      body: { lockVersion: c.lockVersion },
    })
    await refresh()
  } catch (e: any) {
    notice.value = e?.data?.message || '删除失败'
  }
}
</script>

<template>
  <div class="users-page">
    <header class="page-heading">
      <div>
        <h1>挑战管理</h1>
        <p>维护挑战说明、阶段 Flag 和关联靶机模板。</p>
      </div>
      <NuxtLink to="/admin/challenges/new">
        <Plus :size="15" aria-hidden="true" />
        <span>新建挑战</span>
      </NuxtLink>
    </header>
    <AdminDataLoading :pending="pending" :has-data="!!data" />

    <div class="filters">
      <label class="search">
        <Search :size="15" aria-hidden="true" />
        <input v-model="query" aria-label="搜索挑战" placeholder="搜索名称或分类" />
      </label>
      <span class="count muted">{{ rows.length }} 个挑战</span>
    </div>

    <p v-if="error || notice" class="notice" role="alert">
      {{ notice || '读取失败，请刷新重试' }}
    </p>

    <div class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>挑战</th>
            <th>分类</th>
            <th>阶段</th>
            <th>状态</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in rows" :key="c.id">
            <td>
              <NuxtLink :to="`/admin/challenges/${c.id}`" class="user-name">
                {{ c.title }}
              </NuxtLink>
              <span class="secondary">{{ c.summary }}</span>
            </td>
            <td>{{ c.category || '未分类' }}</td>
            <td>{{ c.totalStages }}</td>
            <td>
              <span class="status" :class="{ active: c.status === 'published' }">
                {{ c.status === 'published' ? '已发布' : '草稿' }}
              </span>
            </td>
            <td class="actions">
              <NuxtLink :to="`/admin/challenges/${c.id}`">编辑</NuxtLink>
              <button type="button" @click="remove(c)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !error && !pending" class="empty">暂无挑战</p>
    </div>
  </div>
</template>

<style scoped src="~/assets/css/admin-users.css"></style>
