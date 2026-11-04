<script setup lang="ts">
import { BookOpen, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import type { ReferencePage } from '~/shared/references'
import {
  DEFAULT_SEO_KEYWORDS,
  buildKeywords,
  canonicalUrl
} from '~/shared/seo'
import { useSiteOrigin } from '~/composables/useSiteOrigin'
definePageMeta({ layout: 'learning' })
const route = useRoute()
const page = computed(() => Number(route.query.page || 1))
const siteOrigin = useSiteOrigin()
const pageCanonical = computed(() => {
  const base = canonicalUrl(siteOrigin, '/references')
  return page.value > 1 ? `${base}?page=${page.value}` : base
})
const pageDescription = '公开的网络安全学习参考资料、技术阅读笔记与安全译文。'
useSeoMeta({
  title: () => (page.value > 1 ? `参考资料（第 ${page.value} 页）— AttackBox` : '参考资料 — AttackBox'),
  description: pageDescription,
  ogTitle: () =>
    page.value > 1 ? `参考资料（第 ${page.value} 页）— AttackBox` : '参考资料 — AttackBox',
  ogDescription: pageDescription,
  ogType: 'website',
  ogUrl: () => pageCanonical.value
})
const { data, pending, error } = await useFetch<{ data: ReferencePage }>('/api/references', {
  key: () => `references:catalog:${page.value}`,
  query: { page }
})
useHead(() => ({
  meta: [
    {
      name: 'keywords',
      content: buildKeywords(
        DEFAULT_SEO_KEYWORDS,
        '网络安全参考资料',
        '安全技术译文',
        data.value?.data.items.slice(0, 6).map(i => i.title)
      )
    }
  ],
  link: [{ rel: 'canonical', href: pageCanonical.value }]
}))
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
        <BookOpen :size="20" :stroke-width="1.6" aria-hidden="true" />
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
        ><BookOpen :size="20" :stroke-width="1.5" aria-hidden="true" />
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
  padding: 12px 0 28px;
  border-bottom: 1px solid var(--line);
}
.reference-mark {
  display: inline-grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
  color: var(--brand);
  margin-bottom: 16px;
}
h1 {
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.8px;
  color: var(--ink);
  margin: 0;
}
header p {
  font-size: 13.5px;
  color: var(--muted);
  margin: 10px 0 0;
}
.reference-list {
  margin-top: 22px;
  display: grid;
  gap: 12px;
}
.reference-item {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 22px 24px;
  background: var(--card-surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
  text-decoration: none;
  color: var(--ink);
  transition:
    transform 0.2s cubic-bezier(0.22, 1, 0.36, 1),
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}
.reference-item:hover {
  transform: translateY(-2px);
  border-color: var(--brand-border);
  box-shadow: var(--card-hover-shadow);
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
  font-size: 17.5px;
  font-weight: 650;
  letter-spacing: -0.3px;
  margin: 0;
  overflow-wrap: anywhere;
}
.reference-item p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.75;
  margin: 6px 0 0;
  overflow-wrap: anywhere;
}
.reference-item > span {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  padding: 5px 10px;
  border-radius: 6px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  color: var(--muted);
  transition:
    color 0.16s ease,
    border-color 0.16s ease;
}
.reference-item:hover > span {
  color: var(--brand);
  border-color: var(--brand-border);
}
.reference-empty {
  display: grid;
  justify-items: center;
  gap: 16px;
  margin-top: 22px;
  padding: 80px 20px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
  border-radius: 13px;
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
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: var(--muted);
}
a:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
@media (max-width: 640px) {
  .reference-item {
    padding: 18px 16px;
    gap: 14px;
  }
}
</style>
