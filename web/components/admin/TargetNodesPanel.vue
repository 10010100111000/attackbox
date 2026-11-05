<script setup lang="ts">
import { Server } from 'lucide-vue-next'
import type { ComposePreparation } from '~/shared/compose-labs'

const props = defineProps<{
  isNew: boolean
  dirty: boolean
  nodeBusy: string
  nodes: { id: string; label: string; available: boolean; state: string; bootId: string | null }[]
  preparations: ComposePreparation[]
}>()

const emit = defineEmits<{
  prepare: [nodeId: string, action: 'prepare' | 'status' | 'cancel' | 'rebuild']
}>()

const labels: Record<string, string> = {
  ready: '已准备',
  preparing: '准备中',
  failed: '准备失败',
  unknown: '待确认',
  cancelling: '取消中',
  cancelled: '已取消'
}

function preparation(nodeId: string) {
  return props.preparations.find(p => p.nodeId === nodeId)
}

function preparationActive(nodeId: string) {
  return ['preparing', 'cancelling', 'unknown'].includes(preparation(nodeId)?.state || '')
}

function badgeVariant(node: (typeof props.nodes)[number]) {
  if (!node.available) return 'neutral'
  const state = preparation(node.id)?.state || ''
  if (state === 'ready') return 'success'
  if (state === 'preparing' || state === 'cancelling') return 'brand'
  if (state === 'failed' || state === 'unknown') return 'error'
  return 'neutral'
}
</script>

<template>
  <section class="panel space-y-4">
    <div>
      <h2 class="text-[15px] font-semibold m-0 flex items-center gap-2">
        <Server :size="16" />
        <span>镜像准备</span>
      </h2>
      <p class="text-xs text-[var(--muted)] m-0 mt-1 leading-relaxed">
        在节点上拉取镜像并构建 Dockerfile，准备完成后即可启用关联。
      </p>
    </div>

    <p v-if="isNew" class="text-xs text-[var(--muted)] m-0 py-2">
      保存模板后，可在此拉取或自动准备节点镜像。
    </p>
    <template v-else>
      <p v-if="dirty" class="text-xs text-[var(--muted)] m-0">
        配置已修改，请先保存模板后再准备镜像。
      </p>
      <p v-if="!nodes.length" class="text-xs text-[var(--muted)] m-0">尚未配置可用节点。</p>

      <div class="divide-y divide-[var(--line)]">
        <div
          v-for="node in nodes"
          :key="node.id"
          class="node-row py-3.5 first:pt-0 last:pb-0 space-y-2.5 text-xs"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <strong class="block font-semibold">{{ node.label }}</strong>
              <code class="text-[11px] text-[var(--muted)]">{{ node.id }}</code>
            </div>
            <div class="flex flex-col items-end gap-1">
              <UiBadge :variant="badgeVariant(node)" dot>
                <UiBusyIndicator
                  v-if="
                    node.available &&
                    ['preparing', 'cancelling'].includes(preparation(node.id)?.state || '')
                  "
                  inline
                />
                <span>{{
                  node.available
                    ? labels[preparation(node.id)?.state || ''] || '未准备'
                    : 'Compose 能力不可用'
                }}</span>
              </UiBadge>
              <small
                v-if="
                  node.available &&
                  node.bootId === preparation(node.id)?.bootId &&
                  preparation(node.id)?.deploymentId &&
                  preparation(node.id)?.state !== 'ready'
                "
                class="text-[11px] text-[var(--muted)]"
              >
                现有部署可用
              </small>
            </div>
          </div>

          <p v-if="preparation(node.id)?.error" class="text-xs text-[var(--error-ink)] m-0">
            {{ preparation(node.id)?.error }}
          </p>

          <div class="flex flex-wrap items-center gap-1.5">
            <template v-if="preparationActive(node.id)">
              <UiButton
                variant="secondary"
                size="xs"
                :disabled="!!nodeBusy || !node.available"
                @click="emit('prepare', node.id, 'cancel')"
              >
                {{ preparation(node.id)?.state === 'cancelling' ? '再次取消' : '取消准备' }}
              </UiButton>
            </template>
            <template v-else>
              <UiButton
                v-if="preparation(node.id)?.state !== 'ready'"
                variant="primary"
                size="xs"
                :disabled="dirty || !!nodeBusy || !node.available"
                @click="emit('prepare', node.id, 'prepare')"
              >
                {{ preparation(node.id) ? '重试准备' : '准备部署' }}
              </UiButton>
              <UiButton
                v-if="preparation(node.id)"
                variant="secondary"
                size="xs"
                :disabled="dirty || !!nodeBusy || !node.available"
                @click="emit('prepare', node.id, 'rebuild')"
              >
                强制重建
              </UiButton>
            </template>
            <UiButton
              variant="ghost"
              size="xs"
              :disabled="!!nodeBusy || !node.available"
              @click="emit('prepare', node.id, 'status')"
            >
              查询状态
            </UiButton>
          </div>

          <details v-if="preparation(node.id)?.logs?.length" class="text-xs text-[var(--muted)]">
            <summary class="cursor-pointer select-none">准备记录</summary>
            <ol class="mt-1.5 pl-4 space-y-1 font-mono text-[11px]">
              <li v-for="(line, index) in preparation(node.id)?.logs" :key="index">{{ line }}</li>
            </ol>
          </details>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 24px;
}
</style>
