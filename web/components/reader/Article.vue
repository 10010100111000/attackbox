<script setup lang="ts">
const element = ref<HTMLElement | null>(null)
defineExpose({ element })
</script>

<template>
  <article ref="element" class="reader-article"><slot /></article>
</template>

<style scoped>
.reader-article {
  --markdown-font-size: 17px;

  width: 100%;
  min-width: 0;
  padding: 36px clamp(24px, 3vw, 48px) 64px;
  overflow-wrap: anywhere;
}
@media (max-width: 600px) {
  .reader-article {
    padding: 26px 20px 40px;
  }
}
</style>
