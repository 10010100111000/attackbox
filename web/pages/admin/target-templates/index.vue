<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, RefreshCw, Box } from 'lucide-vue-next'
import type { TargetTemplateList } from '~/shared/target-templates'

definePageMeta({ layout: 'admin' })
useHead({ title: '靶场模板 — AttackBox' })

const input = ref('')
const search = ref('')
const page = ref(1)

const { data, pending, error, refresh } = await useLazyFetch<{ data: TargetTemplateList }>(
  '/api/admin/target-templates',
  {
    key: 'admin:target-templates',
    query: computed(() => ({ search: search.value, page: page.value }))
  }
)

function submit() {
  page.value = 1
  search.value = input.value.trim()
}
</script>

<template>
  <AdminListPage
    title="靶场模板"
    description="集中维护教学与挑战的 Docker Compose 环境配置。"
  >
    <template #actions>
      <UiButton to="/admin/target-templates/new" size="sm">
        <Plus :size="16" aria-hidden="true" />
        <span>新建模板</span>
      </UiButton>
    </template>
    <AdminDataLoading :pending="pending" :has-data="!!data" />

    <form class="filters" @submit.prevent="submit">
      <AdminSearch v-model="input" label="搜索模板" placeholder="搜索模板名称" />
      <UiButton type="submit" variant="secondary" size="sm">搜索</UiButton>
      <UiButton type="button" variant="secondary" size="sm" :disabled="pending" @click="refresh()">
        <RefreshCw :size="14" aria-hidden="true" :class="{ 'animate-spin': pending }" />
        <span>刷新</span>
      </UiButton>
      <span class="count muted">{{ data?.data.total ?? '—' }} 个模板</span>
    </form>

    <UiNotice v-if="error" variant="error" class="mb-4">
      无法读取模板，请检查登录状态或重试。
    </UiNotice>

    <div v-else class="panel table-wrap" :aria-busy="pending">
      <table>
        <thead>
          <tr>
            <th>模板</th>
            <th>访问入口</th>
            <th>状态</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in data?.data.items" :key="item.id">
            <td class="template-cell">
              <NuxtLink :to="`/admin/target-templates/${item.id}`" class="record-title">
                {{ item.name }}
              </NuxtLink>
              <span class="record-summary">{{ item.description }}</span>
            </td>
            <td>
              <div class="flex items-center gap-2">
                <UiBadge variant="neutral">
                  <code>{{ item.settings.entryService || 'web' }}:{{ item.settings.entryPort }}</code>
                </UiBadge>
                <code
                  v-if="item.settings.file && item.settings.file !== 'compose.yml'"
                  class="text-[11px] muted"
                >
                  {{ item.settings.file }}
                </code>
              </div>
            </td>
            <td>
              <UiBadge :variant="item.enabled ? 'success' : 'neutral'" dot>
                {{ item.enabled ? '已启用' : '已停用' }}
              </UiBadge>
            </td>
            <td class="actions">
              <div class="row-actions">
                <UiButton variant="secondary" size="xs" :to="`/admin/target-templates/${item.id}`">
                  编辑
                </UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <UiEmptyState
        v-if="!pending && !data?.data.items.length"
        :title="search ? '没有符合条件的模板' : '还没有靶场模板'"
        :description="search ? '请尝试更换关键词搜索。' : '集中维护教学与挑战环境配置。'"
      >
        <template #icon>
          <Box :size="26" aria-hidden="true" />
        </template>
      </UiEmptyState>
    </div>

    <footer v-if="data" class="pagination">
      <span class="muted">第 {{ page }} 页 · 每页 20 个</span>
      <div>
        <UiButton
          type="button"
          variant="secondary"
          size="xs"
          :disabled="pending || page === 1"
          @click="page--"
        >
          上一页
        </UiButton>
        <UiButton
          type="button"
          variant="secondary"
          size="xs"
          :disabled="pending || page * 20 >= data.data.total"
          @click="page++"
        >
          下一页
        </UiButton>
      </div>
    </footer>
  </AdminListPage>
</template>
