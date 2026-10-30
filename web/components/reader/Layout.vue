<script setup lang="ts">
import { PanelLeft, PanelLeftClose, PanelLeftOpen, ArrowLeft, X } from 'lucide-vue-next'
import {
  DialogRoot,
  DialogTrigger,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose
} from 'reka-ui'
withDefaults(
  defineProps<{
    outlineLabel: string
    outlineTitle: string
    scrollLabel: string
    backTo: string
    backLabel: string
    sidePractice?: boolean
  }>(),
  { sidePractice: false }
)
const collapsed = ref(false)
const mobileOpen = ref(false)
const scrollElement = ref<HTMLElement | null>(null)
const workspaceElement = ref<HTMLElement | null>(null)
const storageKey = 'attackbox-reader-outline-collapsed'
function closeOutline() {
  mobileOpen.value = false
}
function toggle() {
  collapsed.value = !collapsed.value
  try {
    localStorage.setItem(storageKey, String(collapsed.value))
  } catch {
    /* Layout preferences are optional. */
  }
}
function resize() {
  if (window.innerWidth > 900) closeOutline()
}
onMounted(() => {
  try {
    collapsed.value = localStorage.getItem(storageKey) === 'true'
  } catch {
    /* Layout preferences are optional. */
  }
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => window.removeEventListener('resize', resize))
defineExpose({ scrollElement, workspaceElement, closeOutline })
</script>

<template>
  <div class="reader-layout" :class="{ 'outline-collapsed': collapsed }">
    <aside class="reader-sidebar" :aria-label="outlineLabel">
      <div class="outline-controls">
        <span v-if="!collapsed">{{ outlineLabel }}</span>
        <button
          type="button"
          class="outline-toggle"
          :aria-label="`${collapsed ? '展开' : '收起'}${outlineLabel}`"
          :aria-expanded="!collapsed"
          @click="toggle"
        >
          <PanelLeftOpen v-if="collapsed" :size="17" aria-hidden="true" /><PanelLeftClose
            v-else
            :size="17"
            aria-hidden="true"
          />
        </button>
      </div>
      <div class="outline-scroll">
        <p v-if="!collapsed" class="outline-title">{{ outlineTitle }}</p>
        <slot name="outline" :collapsed="collapsed" :close="closeOutline" />
      </div>
    </aside>
    <div class="reader-main">
      <div class="reader-toolbar">
        <div class="toolbar-navigation">
          <DialogRoot v-model:open="mobileOpen">
            <DialogTrigger class="mobile-menu" :aria-label="`打开${outlineLabel}`"
              ><PanelLeft :size="16" aria-hidden="true"
            /></DialogTrigger>
            <DialogPortal>
              <DialogOverlay class="reader-outline-overlay" />
              <DialogContent class="reader-outline-dialog">
                <div class="mobile-outline-header">
                  <DialogTitle>{{ outlineLabel }}</DialogTitle>
                  <DialogClose class="outline-toggle" :aria-label="`关闭${outlineLabel}`"
                    ><X :size="16" aria-hidden="true"
                  /></DialogClose>
                </div>
                <DialogDescription class="sr-only"
                  >选择正文目录，或关闭目录继续阅读。</DialogDescription
                >
                <div class="mobile-outline-scroll">
                  <p class="outline-title">{{ outlineTitle }}</p>
                  <slot name="outline" :collapsed="false" :close="closeOutline" />
                </div>
              </DialogContent>
            </DialogPortal>
          </DialogRoot>
          <NuxtLink :to="backTo" class="reader-back"
            ><ArrowLeft :size="14" aria-hidden="true" />{{ backLabel }}</NuxtLink
          >
        </div>
        <div v-if="$slots.tools" class="reader-tools" role="group" aria-label="阅读工具">
          <slot name="tools" />
        </div>
      </div>
      <div
        ref="workspaceElement"
        class="reader-workspace"
        :class="{ 'practice-on-right': sidePractice }"
      >
        <div ref="scrollElement" class="reader-scroll" tabindex="0" :aria-label="scrollLabel">
          <slot />
        </div>
        <slot name="practice" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.reader-layout {
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
  height: calc(100dvh - 68px);
  min-height: 320px;
  background: var(--reader-bg);
  color: var(--ink);
}
.reader-layout.outline-collapsed {
  grid-template-columns: 56px minmax(0, 1fr);
}
.reader-sidebar {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--line);
  background: var(--paper);
}
.outline-controls {
  height: 45px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 16px;
  border-bottom: 1px solid var(--line);
  font-size: 11px;
  color: var(--muted);
}
.outline-toggle {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--muted);
  cursor: pointer;
}
.outline-toggle:hover {
  background: var(--soft);
  color: var(--ink);
}
.outline-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 20px 16px 28px;
}
.outline-title {
  margin: 0 9px 22px;
  font-size: 15px;
  font-weight: 650;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.outline-collapsed .outline-controls {
  justify-content: center;
  padding: 0;
}
.outline-collapsed .outline-scroll {
  padding-inline: 8px;
}
.reader-main {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
}
.reader-toolbar {
  height: 45px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 24px;
  border-bottom: 1px solid var(--line);
}
.toolbar-navigation,
.reader-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.reader-tools {
  min-height: 32px;
}
.reader-back {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 11px;
  text-decoration: none;
}
.reader-back:hover {
  color: var(--ink);
}
.reader-workspace {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.reader-workspace.practice-on-right {
  flex-direction: row;
}
.reader-scroll {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  overscroll-behavior: contain;
  scroll-padding-top: 24px;
}
.mobile-menu {
  display: none;
}
a:focus-visible,
button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
.reader-scroll:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -2px;
}
@media (max-width: 900px) {
  .reader-layout,
  .reader-layout.outline-collapsed {
    grid-template-columns: minmax(0, 1fr);
  }
  .reader-sidebar {
    display: none;
  }
  .mobile-menu {
    display: grid;
    place-items: center;
    padding: 5px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
}
@media (max-width: 600px) {
  .reader-toolbar {
    padding-inline: 14px;
    gap: 10px;
  }
}
:global(.reader-outline-overlay) {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: #0006;
}
:global(.reader-outline-dialog) {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: min(320px, 85vw);
  z-index: 81;
  display: flex;
  flex-direction: column;
  background: var(--paper);
  color: var(--ink);
  border-right: 1px solid var(--line);
}
:global(.reader-outline-dialog .mobile-outline-header) {
  height: 45px;
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  border-bottom: 1px solid var(--line);
  font-size: 12px;
}
:global(.reader-outline-dialog .mobile-outline-scroll) {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 20px 16px 28px;
}
</style>
