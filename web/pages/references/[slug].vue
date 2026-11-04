<script setup lang="ts">
import ReferenceReader from '~/components/reader/ReferenceReader.vue'
import type { ReferenceReading } from '~/shared/references'
import {
  buildKeywords,
  canonicalUrl,
  serializeJsonLd
} from '~/shared/seo'
import { useSiteOrigin } from '~/composables/useSiteOrigin'
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
const siteOrigin = useSiteOrigin()
const pageCanonical = canonicalUrl(siteOrigin, `/references/${encodeURIComponent(slug)}`)
const pageDescription = meta.summary || `${meta.title} — AttackBox 公开网络安全参考资料。`
const pageKeywords = buildKeywords(
  meta.title,
  meta.outline.slice(0, 8).map(h => h.text),
  '网络安全参考资料',
  '技术笔记',
  'AttackBox'
)
useHead({
  meta: [{ name: 'keywords', content: pageKeywords }],
  link: [{ rel: 'canonical', href: pageCanonical }],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: serializeJsonLd({
        '@context': 'https://schema.org',
        '@graph': [
          {
            '@type': 'TechArticle',
            headline: meta.title,
            description: pageDescription,
            url: pageCanonical,
            inLanguage: 'zh-CN',
            author: {
              '@type': 'Organization',
              name: 'AttackBox',
              url: siteOrigin
            },
            ...(meta.outline.length ? { articleSection: meta.outline.map(h => h.text) } : {})
          },
          {
            '@type': 'BreadcrumbList',
            itemListElement: [
              { '@type': 'ListItem', position: 1, name: '首页', item: `${siteOrigin}/` },
              {
                '@type': 'ListItem',
                position: 2,
                name: '参考资料',
                item: `${siteOrigin}/references`
              },
              {
                '@type': 'ListItem',
                position: 3,
                name: meta.title,
                item: pageCanonical
              }
            ]
          }
        ]
      })
    }
  ]
})
useSeoMeta({
  title: `${meta.title} — 参考资料 — AttackBox`,
  description: pageDescription,
  ogTitle: `${meta.title} — 参考资料 — AttackBox`,
  ogDescription: pageDescription,
  ogUrl: pageCanonical,
  ogType: 'article'
})
</script>
<template><ReferenceReader :key="document.meta.revision" :document="document" /></template>
