<script setup lang="ts">
import LearningHeader from '~/components/learning/LearningHeader.vue'
</script>

<template>
  <div class="learning-layout">
    <div class="learning-backdrop" aria-hidden="true" />
    <LearningHeader />
    <main class="learning-main">
      <slot />
    </main>
  </div>
</template>

<style src="~/assets/css/learning-shell.css"></style>

<style scoped>
.learning-layout {
  min-height: 100dvh;
  background-color: var(--bg);
  color: var(--ink);
  display: flex;
  flex-direction: column;
  position: relative;
  isolation: isolate;
}

.learning-backdrop {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-image:
    var(--page-ambient),
    linear-gradient(to right, var(--bg-grid-line) 1px, transparent 1px),
    linear-gradient(to bottom, var(--bg-grid-line) 1px, transparent 1px);
  background-size: 100% 100%, 28px 28px, 28px 28px;
  mask-image: radial-gradient(ellipse 95% 70% at 50% 0%, #000 35%, transparent 90%);
  -webkit-mask-image: radial-gradient(ellipse 95% 70% at 50% 0%, #000 35%, transparent 90%);
}

.learning-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}
</style>
