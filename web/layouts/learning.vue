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

/* 克制、静谧的顶部暖铜暗影氛围光（与 landing.vue / AuthVisualAside.vue 同源） */
.learning-backdrop {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: radial-gradient(ellipse 75% 320px at 50% -80px, rgba(195, 154, 123, 0.065), transparent 75%);
}

.learning-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}
</style>
