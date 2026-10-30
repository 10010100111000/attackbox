<script setup lang="ts">
import { Plus, RefreshCw, Megaphone } from 'lucide-vue-next'
import { adPlacements, adState, type Advertisement } from '~/shared/advertising'
import { apiErrorMessage, apiFetch } from '~/utils/api'
definePageMeta({ layout: 'admin' })
useHead({ title: '广告管理 — AttackBox' })
const page = ref(1),
  searchInput = ref(''),
  search = ref(''),
  removing = ref(''),
  notice = ref('')
const confirm = useConfirm()
const { data, pending, error, refresh } = await useLazyFetch<{
  data: {
    items: (Advertisement & { impressions: number; clicks: number })[]
    total: number
    pageSize: number
  }
}>('/api/admin/advertisements', {
  key: 'admin:advertisements',
  query: computed(() => ({ page: page.value, search: search.value }))
})
const placementName = (values: string[]) =>
  adPlacements
    .filter(p => values.includes(p.value))
    .map(p => p.label)
    .join('、')
function filter() {
  page.value = 1
  search.value = searchInput.value.trim()
}
async function remove(ad: Advertisement) {
  if (
    removing.value ||
    !(await confirm({
      title: '删除广告？',
      description: `“${ad.name}”将停止展示，已记录的统计保留至保留期结束。`,
      confirmLabel: '删除广告'
    }))
  )
    return
  removing.value = ad.id
  notice.value = ''
  try {
    await apiFetch(`/api/admin/advertisements/${ad.id}`, {
      method: 'DELETE',
      body: { lockVersion: ad.lockVersion }
    })
    await refresh()
  } catch (e) {
    notice.value = apiErrorMessage(e, '删除失败')
  } finally {
    removing.value = ''
  }
}
</script>

<template>
  <AdminListPage title="广告管理" description="管理图文广告与赞助链接，按位置、时间和权重展示。">
    <template #actions
      ><UiButton to="/admin/advertisements/new" size="sm"
        ><Plus :size="16" aria-hidden="true" />新建广告</UiButton
      ></template
    >
    <AdminDataLoading :pending="pending" :has-data="!!data" />
    <form class="filters" @submit.prevent="filter">
      <AdminSearch v-model="searchInput" label="搜索广告" placeholder="搜索名称或标题" />
      <UiButton type="submit" variant="secondary" size="sm">搜索</UiButton>
      <UiButton variant="secondary" size="sm" :disabled="pending" @click="refresh()"
        ><RefreshCw :size="14" aria-hidden="true" />刷新</UiButton
      >
      <span class="count muted">{{ data?.data.total ?? 0 }} 条广告</span>
    </form>
    <UiNotice v-if="notice || error" variant="error" class="mb-4">{{
      notice || '无法读取广告列表'
    }}</UiNotice>
    <div v-else class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>广告</th>
            <th>展示位置</th>
            <th>状态</th>
            <th>权重</th>
            <th>近 30 天曝光／点击</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ad in data?.data.items" :key="ad.id">
            <td>
              <NuxtLink :to="`/admin/advertisements/${ad.id}`" class="record-title">{{
                ad.name
              }}</NuxtLink
              ><span class="record-summary">{{ ad.title }}</span>
            </td>
            <td>{{ placementName(ad.placements) }}</td>
            <td>
              <UiBadge :variant="adState(ad) === '展示中' ? 'success' : 'neutral'" dot>{{
                adState(ad)
              }}</UiBadge>
            </td>
            <td>{{ ad.weight }}</td>
            <td>{{ ad.impressions }} / {{ ad.clicks }}</td>
            <td class="actions">
              <div class="row-actions">
                <UiButton :to="`/admin/advertisements/${ad.id}`" variant="secondary" size="xs"
                  >编辑</UiButton
                ><UiButton
                  variant="danger"
                  size="xs"
                  :loading="removing === ad.id"
                  :disabled="!!removing"
                  @click="remove(ad)"
                  >删除</UiButton
                >
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <UiEmptyState
        v-if="!pending && !data?.data.items.length"
        title="还没有广告"
        description="创建素材并启用后，才会在指定位置展示。"
        ><template #icon><Megaphone :size="26" aria-hidden="true" /></template
      ></UiEmptyState>
    </div>
    <footer v-if="data" class="pagination">
      <span class="muted">第 {{ page }} 页 · 每页 20 条</span>
      <div>
        <UiButton variant="secondary" size="xs" :disabled="pending || page === 1" @click="page--"
          >上一页</UiButton
        ><UiButton
          variant="secondary"
          size="xs"
          :disabled="pending || page * 20 >= data.data.total"
          @click="page++"
          >下一页</UiButton
        >
      </div>
    </footer>
  </AdminListPage>
</template>
