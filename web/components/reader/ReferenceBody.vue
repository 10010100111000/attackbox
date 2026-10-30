<script setup lang="ts">
defineProps<{ chunks: { index: number; html: string }[] }>()
const copy = useMarkdownCopy()
</script>
<template>
  <div class="markdown-body" @click="copy">
    <section
      v-for="chunk in chunks"
      :key="chunk.index"
      class="reference-chunk"
      :data-chunk="chunk.index"
      v-html="chunk.html"
    />
  </div>
</template>
<style src="~/assets/css/markdown.css"></style>
<style scoped>
.reference-chunk {
  content-visibility: auto;
  contain-intrinsic-size: auto 1000px;
}
</style>
