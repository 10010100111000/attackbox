<script setup lang="ts">
import ReaderLayout from '~/components/reader/Layout.vue'
import ReaderArticle from '~/components/reader/Article.vue'
import type { ReferenceDetail } from '~/shared/references'
import type { MarkdownHeading } from '~/utils/markdown'
definePageMeta({ layout: 'learning' })
const route = useRoute(),
  { currentUser } = useAuth()
const slug = computed(() => String(route.params.slug))
const { data, error } = await useFetch<{ data: ReferenceDetail }>(
  () => `/api/references/${slug.value}`,
  { key: `references:document:${slug.value}` }
)
if (error.value)
  throw createError({
    statusCode: error.value.statusCode || 503,
    message: error.value.statusCode === 404 ? '资料不存在或尚未发布' : '暂时无法读取资料'
  })
const item = computed(() => data.value!.data)
useSeoMeta({
  title: () => `${item.value.title} — 参考资料 — AttackBox`,
  description: () => item.value.summary || `${item.value.title}，AttackBox 公开参考资料。`,
  ogTitle: () => item.value.title,
  ogDescription: () => item.value.summary,
  ogType: 'article'
})
const readerLayout = ref<InstanceType<typeof ReaderLayout> | null>(null)
const readerArticle = ref<InstanceType<typeof ReaderArticle> | null>(null)
const scroll = computed(() => readerLayout.value?.scrollElement ?? null)
const article = computed(() => readerArticle.value?.element ?? null)
const sections = ref<MarkdownHeading[]>([])
useReadingAnalytics(scroll)
const identity = computed(() => `reference:${currentUser.value?.id || 'guest'}:${item.value.id}`)
const content = computed(() => item.value.bodyMarkdown)
const { activeSection, goToSection } = useReaderNavigation(
  scroll,
  article,
  identity,
  content,
  sections
)
function headingsLoaded(headings: MarkdownHeading[]) {
  sections.value = headings
}
function jump(id: string) {
  readerLayout.value?.closeOutline()
  void goToSection(id)
}
</script>
<template>
  <ReaderLayout
    ref="readerLayout"
    outline-label="资料目录"
    :outline-title="item.title"
    scroll-label="资料正文"
    back-to="/references"
    back-label="参考资料"
  >
    <template #outline="{ collapsed }">
      <ReaderHeadingTree
        v-show="!collapsed"
        :key="item.id"
        :headings="sections"
        :active-id="activeSection"
        label="本篇目录"
        @select="jump"
      />
    </template>
    <ReaderArticle ref="readerArticle">
      <header class="reference-heading">
        <h1>{{ item.title }}</h1>
        <p v-if="item.summary">{{ item.summary }}</p>
      </header>
      <UiMarkdownView
        :content="item.bodyMarkdown"
        :image-urls="item.imageUrls"
        @headings-loaded="headingsLoaded"
      />
      <AdvertisingSlot :key="item.id" placement="reference_footer" />
    </ReaderArticle>
  </ReaderLayout>
</template>

<style scoped>
.reference-heading {
  padding-bottom: 28px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 28px;
}
h1 {
  font-size: clamp(26px, 2.7vw, 32px);
  font-weight: 700;
  color: var(--ink);
  letter-spacing: -0.6px;
  line-height: 1.4;
  margin: 0;
}
.reference-heading p {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.9;
  margin: 16px 0 0;
}
</style>
