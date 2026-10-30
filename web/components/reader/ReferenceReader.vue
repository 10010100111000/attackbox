<script setup lang="ts">
import ReaderLayout from '~/components/reader/Layout.vue'
import ReaderArticle from '~/components/reader/Article.vue'
import type { ReferenceReading } from '~/shared/references'
const props = withDefaults(
  defineProps<{
    document: ReferenceReading
    preview?: boolean
    status?: string
    backTo?: string
    backLabel?: string
  }>(),
  { preview: false, status: '', backTo: '/references', backLabel: '参考资料' }
)
const document = props.document
const meta = document.meta
const { currentUser } = useAuth()
const layout = ref<InstanceType<typeof ReaderLayout> | null>(null)
const articleRef = ref<InstanceType<typeof ReaderArticle> | null>(null)
const sentinel = ref<HTMLElement | null>(null)
const scroll = computed(() => layout.value?.scrollElement ?? null)
const article = computed(() => articleRef.value?.element ?? null)
const { chunks, busy, failure, outdated, first, complete, ensureSection, previous, retry } =
  useReferenceChunks(document, scroll, sentinel)
const sections = computed(() => meta.outline)
const identity = computed(
  () =>
    `${props.preview ? 'reference-preview' : 'reference'}:${currentUser.value?.id || 'guest'}:${meta.id}`
)
const content = computed(() => chunks.value.map(c => c.index).join(','))
const { activeSection, goToSection } = useReaderNavigation(
  scroll,
  article,
  identity,
  content,
  sections,
  ensureSection
)
if (!props.preview) useReadingAnalytics(scroll)
function jump(id: string) {
  layout.value?.closeOutline()
  void goToSection(id)
}
async function retryLoad() {
  const heading = await retry()
  if (heading) await goToSection(heading)
}
</script>
<template>
  <ReaderLayout
    ref="layout"
    outline-label="资料目录"
    :outline-title="meta.title"
    scroll-label="资料正文"
    :back-to="backTo"
    :back-label="backLabel"
  >
    <template v-if="preview" #tools
      ><span class="preview-label">管理员预览</span
      ><UiBadge :variant="status === 'published' ? 'success' : 'neutral'">{{
        status === 'published' ? '已发布' : '未发布'
      }}</UiBadge></template
    >
    <template #outline="{ collapsed }">
      <ReaderHeadingTree
        v-show="!collapsed"
        :headings="sections"
        :active-id="activeSection"
        label="本篇目录"
        @select="jump"
      />
    </template>
    <ReaderArticle ref="articleRef">
      <header class="reference-heading">
        <h1>{{ meta.title }}</h1>
        <p v-if="meta.summary">{{ meta.summary }}</p>
      </header>
      <div v-if="first > 0" class="previous-content">
        <UiButton variant="secondary" size="sm" :disabled="busy" @click="previous"
          >加载前面的内容</UiButton
        >
      </div>
      <ReaderReferenceBody :chunks="chunks" />
      <div ref="sentinel" class="reading-boundary">
        <UiNotice v-if="failure" variant="error"
          >{{ failure
          }}<UiButton
            v-if="outdated"
            variant="secondary"
            size="xs"
            @click="reloadNuxtApp({ force: true })"
            >重新打开</UiButton
          ><UiButton v-else variant="secondary" size="xs" @click="retryLoad"
            >重试</UiButton
          ></UiNotice
        >
        <UiBusyIndicator v-else-if="busy" label="正文加载" />
      </div>
      <AdvertisingSlot v-if="complete && !preview" :key="meta.id" placement="reference_footer" />
    </ReaderArticle>
  </ReaderLayout>
</template>
<style scoped>
.preview-label {
  color: var(--muted);
  font-size: 12px;
}
.previous-content {
  margin-bottom: 24px;
}
.reading-boundary {
  min-height: 1px;
}
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
