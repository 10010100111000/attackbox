<script setup lang="ts">
import { computed, watch } from 'vue'
import { renderMarkdown, type MarkdownHeading } from '~/utils/markdown'

interface Props {
  content?: string
  imageUrls?: Record<string, string>
  copyable?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  content: '',
  copyable: true
})

const emit = defineEmits<{
  (e: 'headings-loaded', headings: MarkdownHeading[]): void
}>()

const parsed = computed(() => {
  return renderMarkdown(props.content || '', props.imageUrls, { copyCode: props.copyable })
})

watch(
  () => parsed.value.headings,
  headings => {
    emit('headings-loaded', headings)
  },
  { immediate: true }
)
</script>
<template><UiMarkdownContent :html="parsed.html" :copyable="copyable" /></template>
