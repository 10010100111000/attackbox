<script setup lang="ts">
const props = withDefaults(defineProps<{ html: string; copyable?: boolean }>(), { copyable: true })
const handleCopyClick = useMarkdownCopy(() => props.copyable)
</script>
<template>
  <div class="markdown-body" @click="handleCopyClick" v-html="html" />
</template>
<style src="~/assets/css/markdown.css"></style>
