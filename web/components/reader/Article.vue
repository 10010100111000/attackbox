<script setup lang="ts">
const element = ref<HTMLElement | null>(null)
defineExpose({ element })
</script>

<template>
  <article ref="element" class="reader-article"><slot /></article>
</template>

<style scoped>
.reader-article {
  width: 100%;
  min-width: 0;
  padding: 36px clamp(24px, 3vw, 48px) 64px;
  overflow-wrap: anywhere;
}
.reader-article :deep(.markdown-body) {
  color: var(--reader-ink);
  font-size: 17px;
  line-height: 1.9;
}
.reader-article :deep(.markdown-body p) {
  color: var(--reader-ink);
  opacity: 1;
}
.reader-article :deep(.markdown-body h2) {
  color: var(--ink);
  font-size: 22px;
  margin-top: 1.6em;
  border: 0;
  padding-bottom: 0;
}
.reader-article :deep(.markdown-body h3) {
  color: var(--ink);
  font-size: 19px;
}
.reader-article :deep(.markdown-body pre) {
  font-size: 14px;
}
.reader-article :deep(.markdown-body .callout p) {
  font-size: inherit;
}
@media (max-width: 600px) {
  .reader-article {
    padding: 26px 20px 40px;
  }
}
</style>
