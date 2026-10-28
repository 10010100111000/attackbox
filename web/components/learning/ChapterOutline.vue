<script setup lang="ts">
import { NuxtLink } from '#components'
import type { MarkdownHeading } from '~/utils/markdown'
import type { LearningChapter } from '~/shared/learning'
import { LockKeyhole, PanelLeftClose, PanelLeftOpen, CheckCircle2, ChevronDown } from 'lucide-vue-next'
const props = withDefaults(defineProps<{
  moduleSlug: string; moduleTitle: string; currentSlug: string; collapsed?: boolean
  sections?: MarkdownHeading[]; activeSection?: string
  chapters: Pick<LearningChapter, 'id' | 'slug' | 'title' | 'isLocked' | 'lockReason' | 'completedAt'>[]
}>(), { sections: () => [], activeSection: '' })
const expanded = ref(true)
const sectionsId = useId()
watch(() => props.currentSlug, () => { expanded.value = true })
const emit = defineEmits<{ toggle: []; navigate: []; section: [id: string] }>()
</script>

<template>
  <div class="outline-content" :class="{ collapsed }">
    <div class="outline-controls">
      <span v-if="!collapsed">模块目录</span>
      <button type="button" class="outline-toggle" :aria-label="collapsed ? '展开模块目录' : '收起模块目录'" :aria-expanded="!collapsed" @click="emit('toggle')">
        <PanelLeftOpen v-if="collapsed" :size="17" aria-hidden="true" /><PanelLeftClose v-else :size="17" aria-hidden="true" />
      </button>
    </div>
    <h2 v-if="!collapsed">{{ moduleTitle }}</h2>
    <nav aria-label="章节导航">
      <div v-for="(chapter, index) in chapters" :key="chapter.id" class="chapter-entry">
      <div class="chapter-row">
      <component :is="chapter.lockReason === 'prerequisite' ? 'button' : NuxtLink" :disabled="chapter.lockReason === 'prerequisite' || undefined" :type="chapter.lockReason === 'prerequisite' ? 'button' : undefined"
        :to="chapter.lockReason === 'prerequisite' ? undefined : chapter.lockReason === 'login' ? `/auth?returnTo=${encodeURIComponent(`/learn/modules/${moduleSlug}/${chapter.slug}`)}` : `/learn/modules/${moduleSlug}/${chapter.slug}`"
        class="chapter-nav-btn" :class="{ selected: chapter.slug === currentSlug }"
        :aria-current="chapter.slug === currentSlug ? 'page' : undefined"
        :aria-label="collapsed ? `${index + 1}. ${chapter.title}${chapter.lockReason === 'login' ? '，需登录' : chapter.lockReason === 'prerequisite' ? '，需先完成前面的章节' : ''}` : undefined"
        :aria-disabled="chapter.lockReason === 'prerequisite' || undefined" :title="chapter.lockReason === 'prerequisite' ? '需先完成前面的章节' : collapsed ? chapter.title : undefined" @click="chapter.lockReason === 'prerequisite' ? $event.preventDefault() : emit('navigate')">
        <span class="chapter-nav-num">{{ String(index + 1).padStart(2, '0') }}</span>
        <span v-if="!collapsed" class="chapter-nav-text">{{ chapter.title }}<span>{{ chapter.completedAt ? '已完成' : chapter.slug === currentSlug ? '正在阅读' : chapter.lockReason === 'login' ? '需登录' : chapter.lockReason === 'prerequisite' ? '需先完成前章' : '可阅读' }}</span></span>
        <CheckCircle2 v-if="chapter.completedAt && !collapsed" :size="14" class="chapter-done" aria-label="已完成" /><LockKeyhole v-else-if="chapter.isLocked && !collapsed" :size="13" aria-hidden="true" />
      </component>
      <button v-if="chapter.slug === currentSlug && sections.length && !collapsed" class="sections-toggle" type="button" :aria-label="expanded ? '收起本章小节' : '展开本章小节'" :aria-expanded="expanded" :aria-controls="sectionsId" @click="expanded = !expanded"><ChevronDown :size="14" :class="{ folded: !expanded }" aria-hidden="true" /></button>
      </div>
      <ol v-if="chapter.slug === currentSlug && !collapsed" v-show="expanded && sections.length" :id="sectionsId" class="section-list" aria-label="本章小节">
        <li v-for="section in sections" :key="section.id"><a :href="`#${encodeURIComponent(section.id)}`" :aria-current="activeSection === section.id ? 'location' : undefined" @click.prevent="emit('section', section.id); emit('navigate')">{{ section.text }}</a></li>
      </ol>
      </div>
    </nav>
  </div>
</template>

<style scoped>
.outline-content { height: 100%; overflow-y: auto; padding: 16px 16px 28px; }
.outline-controls { display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--muted); font-size: 11px; height: 45px; margin: -16px -16px 24px; padding: 0 16px; border-bottom: 1px solid var(--line); }
.outline-toggle { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; border: 1px solid var(--line); border-radius: 6px; background: var(--paper); color: var(--muted); cursor: pointer; }
.outline-toggle:hover { color: var(--ink); background: var(--soft); }
h2 { font-size: 15px; line-height: 1.6; font-weight: 650; margin: 0 10px 22px; overflow-wrap: anywhere; }
nav { display: grid; gap: 5px; }
.chapter-row { display: flex; align-items: center; min-width: 0; }
.chapter-nav-btn { flex: 1; min-width: 0; width: 100%; border: 0; background: transparent; text-align: left; cursor: pointer; display: flex; align-items: center; gap: 10px; padding: 12px 10px; border-radius: 7px; text-decoration: none; color: var(--muted); }
.chapter-nav-btn:hover { background: var(--hover); color: var(--ink); }
.chapter-nav-btn.selected { background: var(--mint); color: var(--green); }
.chapter-nav-num { font: 11px monospace; flex-shrink: 0; width: 20px; text-align: center; }
.chapter-nav-text { flex: 1; min-width: 0; overflow-wrap: anywhere; font-size: 12px; font-weight: 550; line-height: 1.6; }
.chapter-nav-text > span { display: block; font-size: 10px; font-weight: 400; opacity: .75; margin-top: 3px; }
svg { flex-shrink: 0; }
.sections-toggle { flex-shrink: 0; display: grid; place-items: center; width: 26px; height: 30px; border: 0; border-radius: 5px; color: var(--muted); background: transparent; cursor: pointer; }
.sections-toggle:hover { background: var(--soft); color: var(--ink); }
.sections-toggle .folded { transform: rotate(-90deg); }
.section-list { list-style: none; margin: 4px 0 12px 20px; padding: 0 0 0 16px; border-left: 1px solid var(--line); }
.section-list a { display: block; padding: 7px 8px; border-radius: 5px; color: var(--muted); text-decoration: none; font-size: 11px; line-height: 1.6; overflow-wrap: anywhere; }
.section-list a:hover { color: var(--ink); background: var(--soft); }
.section-list a[aria-current='location'] { color: var(--green); background: var(--mint); font-weight: 600; }
.chapter-done { color: var(--success-ink); }
.chapter-nav-btn[aria-disabled="true"] { opacity: .6; cursor: not-allowed; }
.collapsed { padding: 16px 8px; }
.collapsed .outline-controls { justify-content: center; margin: -16px -8px 24px; padding: 0; }
.collapsed .chapter-nav-btn { justify-content: center; padding: 12px 0; }
a:focus-visible, button:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
</style>
