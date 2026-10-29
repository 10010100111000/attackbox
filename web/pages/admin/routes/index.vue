<script setup lang="ts">
const confirmAction = useConfirm()
import { ref } from 'vue'
import type { RouteSummary } from '~/shared/routes'
import { Plus, Route } from 'lucide-vue-next'

definePageMeta({ layout: 'admin' })
useHead({ title: '学习路线 — AttackBox' })

const { data, pending, error, refresh } = await useLazyFetch<{ data: RouteSummary[] }>('/api/admin/routes', {
  key: 'admin:routes',
})
const notice = ref('')
const deleting = ref('')

async function remove(route: RouteSummary) {
  if (deleting.value) return
  if (!(await confirmAction({title:'删除学习路线？',description:`删除“${route.title}”，模块与学习记录会保留。`,confirmLabel:'删除路线'}))) return
  deleting.value = route.id
  notice.value = ''
  try {
    await $fetch(`/api/admin/routes/${route.id}`, {
      method: 'DELETE',
      body: { lockVersion: route.lockVersion },
    })
    await refresh()
  } catch (e: any) {
    notice.value = e?.data?.message || '删除失败'
  }
  finally { deleting.value = '' }
}
</script>

<template>
  <div class="users-page">
    <header class="page-heading">
      <div>
        <h1>学习路线</h1>
        <p>将学习模块组合为有顺序的学习计划。</p>
      </div>
      <NuxtLink to="/admin/routes/new">
        <Plus :size="16" aria-hidden="true" />
        <span>新建路线</span>
      </NuxtLink>
    </header>
    <AdminDataLoading :pending="pending" :has-data="!!data" />

    <UiNotice v-if="notice || error" variant="error" class="mb-4">
      {{ notice || '无法读取路线' }}
    </UiNotice>

    <div class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>路线</th>
            <th>状态</th>
            <th>模块</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in data?.data" :key="item.id">
            <td>
              <NuxtLink :to="`/admin/routes/${item.id}`" class="user-name">
                {{ item.title }}
              </NuxtLink>
              <span class="secondary">{{ item.summary }}</span>
            </td>
            <td>
              <UiBadge :variant="item.status === 'published' ? 'success' : 'neutral'" dot>
                {{ item.status === 'published' ? '已发布' : '草稿' }}
              </UiBadge>
            </td>
            <td>{{ item.moduleCount }}</td>
            <td class="actions">
              <NuxtLink :to="`/admin/routes/${item.id}`">编辑</NuxtLink>
              <UiButton
                variant="danger"
                size="xs"
                :loading="deleting === item.id"
                :disabled="!!deleting"
                @click="remove(item)"
              >
                删除
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>

      <UiEmptyState
        v-if="!data?.data.length && !error && !pending"
        title="还没有学习路线"
        description="将学习模块组合为有顺序的学习计划。"
      >
        <template #icon>
          <Route :size="26" aria-hidden="true" />
        </template>
      </UiEmptyState>
    </div>
  </div>
</template>

