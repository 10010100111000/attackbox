<script setup lang="ts">
import ReferenceReader from '~/components/reader/ReferenceReader.vue'
import type { ReferenceReading } from '~/shared/references'
definePageMeta({ layout: 'learning' })
const route = useRoute()
const slug = String(route.params.slug)
const { data, error } = await useFetch<{ data: ReferenceReading }>(`/api/references/${slug}`, {
  key: `references:document:${slug}`
})
if (error.value || !data.value)
  throw createError({
    statusCode: error.value?.statusCode || 503,
    message: error.value?.statusCode === 404 ? '资料不存在或尚未发布' : '暂时无法读取资料'
  })
const document = data.value!.data
const meta = document.meta
useSeoMeta({
  title: `${meta.title} — 参考资料 — AttackBox`,
  description: meta.summary || `${meta.title}，AttackBox 公开参考资料。`,
  ogTitle: meta.title,
  ogDescription: meta.summary,
  ogType: 'article'
})
</script>
<template><ReferenceReader :key="document.meta.revision" :document="document" /></template>
