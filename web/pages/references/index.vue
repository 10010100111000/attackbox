<script setup lang="ts">
import { BookOpen, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import type { ReferencePage } from '~/shared/references'
definePageMeta({ layout: 'learning' })
useSeoMeta({
  title: '参考资料 — AttackBox',
  description: '公开的安全学习参考资料、阅读笔记与译文。'
})
const route = useRoute()
const page = computed(() => Number(route.query.page || 1))
const { data, pending, error } = await useFetch<{ data: ReferencePage }>('/api/references', {
  key: 'references:catalog',
  query: { page }
})
if (import.meta.server && error.value) {
  const event = useRequestEvent()
  if (event) setResponseStatus(event, error.value.statusCode || 503)
}
const result = computed(() => data.value?.data)
</script>
<template>
  <div class="learning-frame learning-shell references-index">
    <header>
      <div class="reference-mark">
        <BookOpen :size="22" :stroke-width="1.5" aria-hidden="true" />
      </div>
      <h1>参考资料</h1>
      <p>留一份时间，读得更深一点。</p>
    </header>
    <div v-if="error" class="reference-empty" role="alert">资料暂时无法展示，请稍后再试。</div>
    <UiBusyIndicator v-else-if="pending && !result" class="reference-empty" label="资料加载" />
    <div v-else-if="!result?.items.length" class="reference-empty">
      <BookOpen :size="30" :stroke-width="1.2" aria-hidden="true" />
      <p>书架还空着，新内容正在路上。</p>
    </div>
    <div v-else class="reference-list">
      <NuxtLink
        v-for="item in result.items"
        :key="item.id"
        :to="`/references/${item.slug}`"
        class="reference-item"
        ><BookOpen :size="20" :stroke-width="1.4" aria-hidden="true" />
        <div>
          <h2>{{ item.title }}</h2>
          <p v-if="item.summary">{{ item.summary }}</p>
        </div>
        <span>阅读</span></NuxtLink
      >
    </div>
    <nav
      v-if="result && result.total > result.pageSize"
      class="reference-pages"
      aria-label="资料分页"
    >
      <NuxtLink v-if="page > 1" :to="{ query: { page: page - 1 } }" aria-label="上一页"
        ><ChevronLeft :size="16" /></NuxtLink
      ><span>{{ page }} / {{ Math.ceil(result.total / result.pageSize) }}</span
      ><NuxtLink
        v-if="page * result.pageSize < result.total"
        :to="{ query: { page: page + 1 } }"
        aria-label="下一页"
        ><ChevronRight :size="16"
      /></NuxtLink>
    </nav>
  </div>
</template>
<style scoped>
header {
  padding: 16px 0 34px;
  border-bottom: 1px solid var(--line);
}
.reference-mark {
  color: var(--brand);
  margin-bottom: 18px;
}
h1 {
  font-size: 30px;
  font-weight: 650;
  letter-spacing: -0.8px;
  color: var(--ink);
  margin: 0;
}
header p {
  font-size: 13px;
  color: var(--muted);
  margin: 12px 0 0;
}
.reference-list {
  margin-top: 12px;
}
.reference-item {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 28px 20px;
  border-bottom: 1px solid var(--line);
  text-decoration: none;
  color: var(--ink);
  border-radius: 8px;
  transition: background 0.18s;
}
.reference-item:hover {
  background: var(--soft);
}
.reference-item > svg {
  color: var(--brand);
  flex-shrink: 0;
}
.reference-item > div {
  flex: 1;
  min-width: 0;
}
h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  overflow-wrap: anywhere;
}
.reference-item p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.8;
  margin: 8px 0 0;
  overflow-wrap: anywhere;
}
.reference-item > span {
  font-size: 11px;
  color: var(--muted);
}
.reference-empty {
  display: grid;
  justify-items: center;
  gap: 16px;
  padding: 90px 20px;
  color: var(--muted);
  font-size: 13px;
}
.reference-empty svg {
  color: var(--brand);
}
.reference-pages {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 24px;
  margin-top: 32px;
  font-size: 12px;
  color: var(--muted);
}
a:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
@media (max-width: 640px) {
  .reference-item {
    padding: 24px 4px;
    gap: 16px;
  }
}
</style>
