<script setup lang="ts">
import { BookOpen } from 'lucide-vue-next'
import HeroSection from '~/components/landing/HeroSection.vue'
import FeaturedModule from '~/components/landing/FeaturedModule.vue'
import type { PublicModule } from '~/shared/module-catalog'
import ModuleRail from '~/components/landing/ModuleRail.vue'

definePageMeta({
  layout: 'landing'
})
// Await SSR data so crawlers and visitors receive the same real module links.
const { data, error } = await useFetch<{ data: PublicModule[] }>('/api/landing', { key: 'landing:modules' })
if (import.meta.server && error.value) {
  const event = useRequestEvent()
  if (event) setResponseStatus(event, 503)
}
const modules = computed(() => data.value?.data || [])
useSeoMeta({
  title: 'AttackBox — 理解原理，亲手验证。',
  description: '从教程到实践，在 AttackBox 学习网络安全。浏览学习模块，阅读公开章节，在终端和实验环境中亲手验证。',
  ogTitle: 'AttackBox — 理解原理，亲手验证。',
  ogDescription: '从教程到实践，探索网络安全。',
  ogType: 'website',
})
</script>

<template>
  <div>
    <HeroSection />
    <section id="modules" class="modules wrap">
      <div class="section-head">
        <h2>下一步，动手。</h2>
      </div>
      <template v-if="modules.length">
        <FeaturedModule :module="modules[0]!" />
        <ModuleRail v-if="modules.length > 1" :modules="modules.slice(1)" />
      </template>
      <div v-else class="catalog-empty">
        <BookOpen :size="28" :stroke-width="1.3" aria-hidden="true" />
        <p>{{ error ? '模块暂时无法展示' : '新的内容，即将开始。' }}</p>
      </div>
      <div class="catalog-link"><NuxtLink to="/learn">浏览学习模块</NuxtLink></div>
    </section>
  </div>
</template>
