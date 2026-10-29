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
const { data, error } = await useFetch<{ data: PublicModule[] }>('/api/landing', {
  key: 'landing:modules'
})
if (import.meta.server && error.value) {
  const event = useRequestEvent()
  if (event) setResponseStatus(event, 503)
}
const modules = computed(() => data.value?.data || [])
useSeoMeta({
  title: 'AttackBox — 理解原理，亲手验证。',
  description:
    '从教程到实践，在 AttackBox 学习网络安全。浏览学习模块，阅读公开章节，在终端和实验环境中亲手验证。',
  ogTitle: 'AttackBox — 理解原理，亲手验证。',
  ogDescription: '从教程到实践，探索网络安全。',
  ogType: 'website'
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

<style scoped>
.modules {
  padding: 55px 0 74px;
  position: relative;
}

.modules:before {
  content: '';
  position: absolute;
  top: 0;
  left: 20%;
  right: 20%;
  height: 1px;
  background: linear-gradient(90deg, transparent, #c19e7f33, transparent);
}

.section-head {
  text-align: center;
  margin-bottom: 38px;
}

.section-head h2 {
  font-size: 34px;
  font-weight: 550;
  letter-spacing: -1.3px;
}

.catalog-empty {
  display: grid;
  justify-items: center;
  gap: 18px;
  padding: 60px 24px;
  color: var(--text-muted);
  border: 1px solid var(--border-line);
  border-radius: 11px;
  background: radial-gradient(ellipse at 50% 100%, var(--brand-warm-soft), transparent 70%);
}

.catalog-empty :deep(svg) {
  color: var(--brand-warm);
}

.catalog-empty p {
  font-size: 14px;
}

.catalog-link {
  display: flex;
  justify-content: center;
  margin-top: 30px;
}

.catalog-link a {
  color: var(--brand-warm);
  font-size: 12px;
  padding: 10px 16px;
  border-radius: 7px;
  border: 1px solid var(--border-line);
}

.catalog-link a:hover {
  background: var(--brand-warm-soft);
}

@media (max-width: 760px) {
  .modules {
    padding: 35px 0 42px;
  }
  .section-head {
    margin-bottom: 27px;
  }
  .section-head h2 {
    font-size: 27px;
  }
}
</style>
