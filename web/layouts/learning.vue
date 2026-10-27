<script setup lang="ts">
import LearningHeader from '~/components/learning/LearningHeader.vue'
</script>

<template>
  <div class="learning-layout">
    <LearningHeader />
    <main class="learning-main">
      <slot />
    </main>
  </div>
</template>

<style src="~/assets/css/learning-shell.css"></style>

<style scoped>
.learning-layout {
  min-height: 100vh;
  background-color: var(--bg);
  color: var(--ink);
  display: flex;
  flex-direction: column;
}

.learning-main {
  flex: 1;
  display: flex;
  flex-direction: column;
}
</style>
