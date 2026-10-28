<script setup lang="ts">
import { ArrowLeft, List } from 'lucide-vue-next'
import type { ReferenceDetail } from '~/shared/references'
import type { MarkdownHeading } from '~/utils/markdown'
definePageMeta({ layout: 'learning' })
const route = useRoute(), { currentUser } = useAuth()
const slug = computed(() => String(route.params.slug))
const { data, error } = await useFetch<{ data: ReferenceDetail }>(() => `/api/references/${slug.value}`, {key:`references:document:${slug.value}`})
if (error.value) throw createError({ statusCode: error.value.statusCode || 503, message: error.value.statusCode === 404 ? '资料不存在或尚未发布' : '暂时无法读取资料' })
const item = computed(() => data.value!.data)
useSeoMeta({ title: () => `${item.value.title} — 参考资料 — AttackBox`, description: () => item.value.summary || `${item.value.title}，AttackBox 公开参考资料。`, ogTitle: () => item.value.title, ogDescription: () => item.value.summary, ogType: 'article' })
const scroll = ref<HTMLElement|null>(null), article = ref<HTMLElement|null>(null), sections = ref<MarkdownHeading[]>([]), mobileToc = ref<HTMLDetailsElement|null>(null)
const identity = computed(() => `reference:${currentUser.value?.id || 'guest'}:${item.value.id}`)
const content = computed(() => item.value.bodyMarkdown)
const { activeSection, goToSection } = useReaderNavigation(scroll, article, identity, content, sections)
function headingsLoaded(headings: MarkdownHeading[]) { sections.value = headings.filter(h => h.level === 2 || h.level === 3) }
function jump(id: string) { if (mobileToc.value) mobileToc.value.open = false; void goToSection(id) }
</script>
<template>
  <div class="reference-reader">
    <aside class="reference-toc" aria-label="资料目录"><NuxtLink to="/references" class="reference-back"><ArrowLeft :size="14" aria-hidden="true"/>参考资料</NuxtLink><nav aria-label="本篇目录"><a v-for="section in sections" :key="section.id" :href="`#${encodeURIComponent(section.id)}`" :class="{'subsection':section.level===3}" :aria-current="activeSection===section.id?'location':undefined" @click.prevent="jump(section.id)">{{section.text}}</a></nav></aside>
    <div ref="scroll" class="reference-scroll" tabindex="0" aria-label="资料正文"><article ref="article" class="reference-article">
      <NuxtLink to="/references" class="reference-back inline-back"><ArrowLeft :size="14" aria-hidden="true"/>参考资料</NuxtLink>
      <header><h1>{{item.title}}</h1><p v-if="item.summary">{{item.summary}}</p></header>
      <details v-if="sections.length" ref="mobileToc" class="mobile-reference-toc"><summary><List :size="14" aria-hidden="true"/>本篇目录</summary><nav><a v-for="section in sections" :key="section.id" :href="`#${encodeURIComponent(section.id)}`" @click.prevent="jump(section.id)">{{section.text}}</a></nav></details>
      <UiMarkdownView :content="item.bodyMarkdown" :image-urls="item.imageUrls" @headings-loaded="headingsLoaded"/>
    </article></div>
  </div>
</template>
<style scoped>
.reference-reader { display: grid; grid-template-columns: 240px minmax(0,1fr); height: calc(100dvh - 68px); min-height: 320px; color: var(--reader-ink); background: var(--reader-bg); }
.reference-toc { padding: 26px 20px; overflow: auto; border-right: 1px solid var(--line); background: var(--paper); }
.reference-back { display: inline-flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); text-decoration: none; }
.reference-back:hover { color: var(--ink); }
.reference-toc nav { display: grid; gap: 4px; margin-top: 28px; }
.reference-toc nav a { padding: 9px 10px; color: var(--muted); text-decoration: none; font-size: 12px; line-height: 1.6; border-radius: 5px; overflow-wrap: anywhere; }
.reference-toc nav a.subsection { padding-left: 24px; font-size: 11px; }
.reference-toc nav a:hover { background: var(--soft); color: var(--ink); }
.reference-toc nav a[aria-current='location'] { color: var(--brand); background: var(--brand-soft); }
.reference-scroll { min-width: 0; min-height: 0; overflow: auto; overscroll-behavior: contain; }
.reference-article { max-width: 960px; margin: auto; padding: 48px clamp(28px,5vw,80px) 100px; overflow-wrap: anywhere; }
.reference-article header { padding-bottom: 30px; border-bottom: 1px solid var(--line); margin-bottom: 30px; }
h1 { font-size: clamp(26px,3vw,34px); font-weight: 650; color: var(--ink); letter-spacing: -.8px; line-height: 1.4; margin: 0; }
header p { color: var(--muted); font-size: 14px; line-height: 1.9; margin: 16px 0 0; }
.reference-article :deep(.markdown-body) { font-size: 17px; line-height: 1.95; color: var(--reader-ink); }
.reference-article :deep(.markdown-body p) { color: inherit; opacity: 1; }
.reference-article :deep(.markdown-body h2) { font-size: 23px; color: var(--ink); border: 0; }
.inline-back { margin-bottom: 26px; }
.mobile-reference-toc { display: none; }
a:focus-visible, summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
@media(min-width:901px) { .reference-reader .inline-back { display: none; } }
@media(max-width:900px) { .reference-reader { grid-template-columns:minmax(0,1fr); }.reference-toc{display:none}.reference-article{padding:28px 24px 64px}.mobile-reference-toc{display:block;margin-bottom:24px;border:1px solid var(--line);border-radius:8px;padding:12px}.mobile-reference-toc summary{display:flex;align-items:center;gap:8px;cursor:pointer;font-size:12px}.mobile-reference-toc nav{display:grid;gap:12px;padding-top:16px}.mobile-reference-toc a{color:var(--muted);font-size:12px;text-decoration:none} }
</style>
