<script setup lang="ts">
const element = ref<HTMLElement | null>(null)
defineExpose({ element })
</script>

<template>
  <article ref="element" class="reader-article"><slot /></article>
</template>

<style scoped>
.reader-article {
  --markdown-font-size: 16.5px;
  --reader-title-size: clamp(28px, 2.8vw, 36px);

  width: 100%;
  max-width: 1100px;
  margin-inline: auto;
  min-width: 0;
  padding: 40px clamp(24px, 4vw, 56px) 72px;
  overflow-wrap: anywhere;
}
@media (max-width: 600px) {
  .reader-article {
    padding: 26px 20px 40px;
  }
}
</style>
