<script setup lang="ts">
import { NuxtLink } from '#components'
import type { LearningChapter } from '~/shared/learning'
import { LockKeyhole, PanelLeftClose, PanelLeftOpen, CheckCircle2 } from 'lucide-vue-next'
defineProps<{
  moduleSlug: string; moduleTitle: string; currentSlug: string; collapsed?: boolean
  chapters: Pick<LearningChapter, 'id' | 'slug' | 'title' | 'isLocked' | 'lockReason' | 'completedAt'>[]
}>()
const emit = defineEmits<{ toggle: []; navigate: [] }>()
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
      <component :is="chapter.lockReason === 'prerequisite' ? 'button' : NuxtLink" :disabled="chapter.lockReason === 'prerequisite' || undefined" :type="chapter.lockReason === 'prerequisite' ? 'button' : undefined" v-for="(chapter, index) in chapters" :key="chapter.id"
        :to="chapter.lockReason === 'prerequisite' ? undefined : chapter.lockReason === 'login' ? `/auth?returnTo=${encodeURIComponent(`/learn/modules/${moduleSlug}/${chapter.slug}`)}` : `/learn/modules/${moduleSlug}/${chapter.slug}`"
        class="chapter-nav-btn" :class="{ selected: chapter.slug === currentSlug }"
        :aria-current="chapter.slug === currentSlug ? 'page' : undefined"
        :aria-label="collapsed ? `${index + 1}. ${chapter.title}${chapter.lockReason === 'login' ? '，需登录' : chapter.lockReason === 'prerequisite' ? '，需先完成前面的章节' : ''}` : undefined"
        :aria-disabled="chapter.lockReason === 'prerequisite' || undefined" :title="chapter.lockReason === 'prerequisite' ? '需先完成前面的章节' : collapsed ? chapter.title : undefined" @click="chapter.lockReason === 'prerequisite' ? $event.preventDefault() : emit('navigate')">
        <span class="chapter-nav-num">{{ String(index + 1).padStart(2, '0') }}</span>
        <span v-if="!collapsed" class="chapter-nav-text">{{ chapter.title }}<span>{{ chapter.completedAt ? '已完成' : chapter.slug === currentSlug ? '正在阅读' : chapter.lockReason === 'login' ? '需登录' : chapter.lockReason === 'prerequisite' ? '需先完成前章' : '可阅读' }}</span></span>
        <CheckCircle2 v-if="chapter.completedAt && !collapsed" :size="14" class="chapter-done" aria-label="已完成" /><LockKeyhole v-else-if="chapter.isLocked && !collapsed" :size="13" aria-hidden="true" />
      </component>
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
.chapter-nav-btn { width: 100%; border: 0; background: transparent; text-align: left; cursor: pointer; display: flex; align-items: center; gap: 10px; padding: 12px 10px; border-radius: 7px; text-decoration: none; color: var(--muted); }
.chapter-nav-btn:hover { background: var(--hover); color: var(--ink); }
.chapter-nav-btn.selected { background: var(--mint); color: var(--green); }
.chapter-nav-num { font: 11px monospace; flex-shrink: 0; width: 20px; text-align: center; }
.chapter-nav-text { flex: 1; min-width: 0; overflow-wrap: anywhere; font-size: 12px; font-weight: 550; line-height: 1.6; }
.chapter-nav-text > span { display: block; font-size: 10px; font-weight: 400; opacity: .75; margin-top: 3px; }
svg { flex-shrink: 0; }
.chapter-done { color: var(--success-ink); }
.chapter-nav-btn[aria-disabled="true"] { opacity: .6; cursor: not-allowed; }
.collapsed { padding: 16px 8px; }
.collapsed .outline-controls { justify-content: center; margin: -16px -8px 24px; padding: 0; }
.collapsed .chapter-nav-btn { justify-content: center; padding: 12px 0; }
a:focus-visible, button:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
</style>
