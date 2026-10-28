<script setup lang="ts">
import { Toggle } from 'reka-ui'
import { Sun, Moon } from 'lucide-vue-next'

const { currentTheme, setTheme } = useTheme()
</script>

<template>
  <Toggle
    class="theme-toggle"
    :model-value="currentTheme === 'midnight'"
    aria-label="深色外观"
    @update:model-value="setTheme($event ? 'midnight' : 'graphite')"
  >
    <Moon v-if="currentTheme === 'midnight'" :size="18" aria-hidden="true" />
    <Sun v-else :size="18" aria-hidden="true" />
  </Toggle>
</template>

<style scoped>
.theme-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.theme-toggle:hover { background: var(--soft); color: var(--ink); }
.theme-toggle:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
</style>
