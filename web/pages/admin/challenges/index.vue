<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus } from 'lucide-vue-next'
import type { ChallengeSummary } from '~/shared/challenges'
import { apiErrorMessage, apiFetch } from '~/utils/api'
const confirmAction = useConfirm()

definePageMeta({ layout: 'admin' })
useHead({ title: '挑战管理 — AttackBox' })

const { data, pending, error, refresh } = await useLazyFetch<{ data: ChallengeSummary[] }>(
  '/api/admin/challenges',
  {
    key: 'admin:challenges'
  }
)

const query = ref('')
const notice = ref('')
const deleting = ref('')

const rows = computed(
  () =>
    data.value?.data.filter(c =>
      `${c.title} ${c.category}`.toLowerCase().includes(query.value.toLowerCase())
    ) || []
)

async function remove(c: ChallengeSummary) {
  if (deleting.value) return
  if (
    !(await confirmAction({
      title: '删除挑战？',
      description: `“${c.title}”及其阶段完成记录将被永久删除。`,
      confirmLabel: '删除挑战'
    }))
  )
    return
  deleting.value = c.id
  notice.value = ''
  try {
    await apiFetch(`/api/admin/challenges/${c.id}`, {
      method: 'DELETE',
      body: { lockVersion: c.lockVersion }
    })
    await refresh()
  } catch (e) {
    notice.value = apiErrorMessage(e, '删除失败')
  } finally {
    deleting.value = ''
  }
}
</script>

<template>
  <AdminListPage title="挑战管理" description="维护挑战说明、阶段 Flag 和关联靶机模板。">
    <template #actions
      ><UiButton to="/admin/challenges/new" size="sm"
        ><Plus :size="16" aria-hidden="true" />新建挑战</UiButton
      ></template
    >
    <AdminDataLoading :pending="pending" :has-data="!!data" />

    <div class="filters">
      <AdminSearch v-model="query" label="搜索挑战" placeholder="搜索名称或分类" />
      <span class="count muted">{{ rows.length }} 个挑战</span>
    </div>

    <UiNotice v-if="error || notice" variant="error" class="mb-4">
      {{ notice || '读取失败，请刷新重试' }}
    </UiNotice>

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
              <NuxtLink :to="`/admin/challenges/${c.id}`" class="record-title">
                {{ c.title }}
              </NuxtLink>
              <span class="record-summary">{{ c.summary }}</span>
            </td>
            <td>{{ c.category || '未分类' }}</td>
            <td>{{ c.totalStages }}</td>
            <td>
              <UiBadge :variant="c.status === 'published' ? 'success' : 'neutral'" dot>
                {{ c.status === 'published' ? '已发布' : '草稿' }}
              </UiBadge>
            </td>
            <td class="actions">
              <div class="row-actions">
                <UiButton variant="secondary" size="xs" :to="`/admin/challenges/${c.id}`"
                  >编辑</UiButton
                >
                <UiButton
                  variant="danger"
                  size="xs"
                  :loading="deleting === c.id"
                  :disabled="!!deleting"
                  @click="remove(c)"
                >
                  删除
                </UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <UiEmptyState
        v-if="!rows.length && !error && !pending"
        title="暂无挑战"
        description="尚未创建任何挑战，点击右上角新建挑战。"
      />
    </div>
  </AdminListPage>
</template>
