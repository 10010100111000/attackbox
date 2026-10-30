<script setup lang="ts">
import { NuxtLink } from '#components'
import type { MarkdownHeading } from '~/utils/markdown'
import type { LearningChapter } from '~/shared/learning'
import { LockKeyhole, CheckCircle2, ChevronDown } from 'lucide-vue-next'
const props = withDefaults(
  defineProps<{
    moduleSlug: string
    currentSlug: string
    collapsed?: boolean
    sections?: MarkdownHeading[]
    activeSection?: string
    chapters: Pick<
      LearningChapter,
      'id' | 'slug' | 'title' | 'isLocked' | 'lockReason' | 'completedAt'
    >[]
  }>(),
  { sections: () => [], activeSection: '' }
)
const expanded = ref(true)
const sectionsId = useId()
watch(
  () => props.currentSlug,
  () => {
    expanded.value = true
  }
)
const emit = defineEmits<{ navigate: []; section: [id: string] }>()
function selectSection(id: string) {
  emit('section', id)
  emit('navigate')
}
</script>

<template>
  <nav class="chapter-outline" :class="{ collapsed }" aria-label="章节导航">
    <div v-for="(chapter, index) in chapters" :key="chapter.id" class="chapter-entry">
      <div class="chapter-row">
        <component
          :is="chapter.lockReason === 'prerequisite' ? 'button' : NuxtLink"
          :disabled="chapter.lockReason === 'prerequisite' || undefined"
          :type="chapter.lockReason === 'prerequisite' ? 'button' : undefined"
          :to="
            chapter.lockReason === 'prerequisite'
              ? undefined
              : chapter.lockReason === 'login'
                ? `/auth?returnTo=${encodeURIComponent(`/learn/modules/${moduleSlug}/${chapter.slug}`)}`
                : `/learn/modules/${moduleSlug}/${chapter.slug}`
          "
          class="chapter-nav-btn"
          :class="{ selected: chapter.slug === currentSlug }"
          :aria-current="chapter.slug === currentSlug ? 'page' : undefined"
          :aria-label="
            collapsed
              ? `${index + 1}. ${chapter.title}${chapter.lockReason === 'login' ? '，需登录' : chapter.lockReason === 'prerequisite' ? '，需先完成前面的章节' : ''}`
              : undefined
          "
          :aria-disabled="chapter.lockReason === 'prerequisite' || undefined"
          :title="
            chapter.lockReason === 'prerequisite'
              ? '需先完成前面的章节'
              : collapsed
                ? chapter.title
                : undefined
          "
          @click="
            chapter.lockReason === 'prerequisite' ? $event.preventDefault() : emit('navigate')
          "
        >
          <span class="chapter-nav-num">{{ String(index + 1).padStart(2, '0') }}</span>
          <span v-if="!collapsed" class="chapter-nav-text"
            >{{ chapter.title
            }}<span>{{
              chapter.completedAt
                ? '已完成'
                : chapter.slug === currentSlug
                  ? '正在阅读'
                  : chapter.lockReason === 'login'
                    ? '需登录'
                    : chapter.lockReason === 'prerequisite'
                      ? '需先完成前章'
                      : '可阅读'
            }}</span></span
          >
          <CheckCircle2
            v-if="chapter.completedAt && !collapsed"
            :size="14"
            class="chapter-done"
            aria-label="已完成"
          /><LockKeyhole v-else-if="chapter.isLocked && !collapsed" :size="13" aria-hidden="true" />
        </component>
        <button
          v-if="chapter.slug === currentSlug && sections.length && !collapsed"
          class="sections-toggle"
          type="button"
          :aria-label="expanded ? '收起本章小节' : '展开本章小节'"
          :aria-expanded="expanded"
          :aria-controls="sectionsId"
          @click="expanded = !expanded"
        >
          <ChevronDown :size="14" :class="{ folded: !expanded }" aria-hidden="true" />
        </button>
      </div>
      <div
        v-if="chapter.slug === currentSlug"
        v-show="!collapsed && expanded && sections.length"
        :id="sectionsId"
        class="section-list"
      >
        <ReaderHeadingTree
          :key="currentSlug"
          :headings="sections"
          :active-id="activeSection"
          label="本章小节"
          @select="selectSection"
        />
      </div>
    </div>
  </nav>
</template>

<style scoped>
.chapter-outline {
  display: grid;
  gap: 5px;
}
.chapter-row {
  display: flex;
  align-items: center;
  min-width: 0;
}
.chapter-nav-btn {
  flex: 1;
  min-width: 0;
  width: 100%;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 10px;
  border-radius: 7px;
  text-decoration: none;
  color: var(--muted);
}
.chapter-nav-btn:hover {
  background: var(--hover);
  color: var(--ink);
}
.chapter-nav-btn.selected {
  background: var(--mint);
  color: var(--green);
}
.chapter-nav-num {
  font: 11px monospace;
  flex-shrink: 0;
  width: 20px;
  text-align: center;
}
.chapter-nav-text {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 12px;
  font-weight: 550;
  line-height: 1.6;
}
.chapter-nav-text > span {
  display: block;
  font-size: 10px;
  font-weight: 400;
  opacity: 0.75;
  margin-top: 3px;
}
svg {
  flex-shrink: 0;
}
.sections-toggle {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  width: 26px;
  height: 30px;
  border: 0;
  border-radius: 5px;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}
.sections-toggle:hover {
  background: var(--soft);
  color: var(--ink);
}
.sections-toggle .folded {
  transform: rotate(-90deg);
}
.section-list {
  list-style: none;
  margin: 4px 0 12px 20px;
  padding: 0 0 0 8px;
  border-left: 1px solid var(--line);
}
.chapter-done {
  color: var(--success-ink);
}
.chapter-nav-btn[aria-disabled='true'] {
  opacity: 0.6;
  cursor: not-allowed;
}
.collapsed .chapter-nav-btn {
  justify-content: center;
  padding: 12px 0;
}
a:focus-visible,
button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -2px;
}
</style>
