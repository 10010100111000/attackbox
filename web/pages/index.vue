<script setup lang="ts">
import { BookOpen } from 'lucide-vue-next'
import HeroSection from '~/components/landing/HeroSection.vue'
import FeaturedModule from '~/components/landing/FeaturedModule.vue'
import type { PublicModule } from '~/shared/module-catalog'
import {
  DEFAULT_SEO_DESCRIPTION,
  DEFAULT_SEO_KEYWORDS,
  SITE_NAME,
  buildKeywords,
  canonicalUrl,
  serializeJsonLd
} from '~/shared/seo'
import { useSiteOrigin } from '~/composables/useSiteOrigin'
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
const siteOrigin = useSiteOrigin()
const homeCanonical = canonicalUrl(siteOrigin, '/')
const homeKeywords = computed(() =>
  buildKeywords(
    DEFAULT_SEO_KEYWORDS,
    modules.value.slice(0, 6).map(m => m.title),
    modules.value.map(m => m.category?.name)
  )
)
const homeJsonLd = computed(() =>
  serializeJsonLd({
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'WebSite',
        '@id': `${siteOrigin}/#website`,
        name: SITE_NAME,
        url: homeCanonical,
        inLanguage: 'zh-CN',
        description: DEFAULT_SEO_DESCRIPTION
      },
      {
        '@type': 'EducationalOrganization',
        '@id': `${siteOrigin}/#organization`,
        name: SITE_NAME,
        url: homeCanonical,
        description: '面向中文学习者的交互式网络安全与实战靶场学习平台。'
      },
      ...(modules.value.length
        ? [
            {
              '@type': 'ItemList',
              name: '精选网络安全学习模块',
              itemListElement: modules.value.slice(0, 6).map((mod, idx) => ({
                '@type': 'ListItem',
                position: idx + 1,
                name: mod.title,
                url: canonicalUrl(siteOrigin, `/learn/modules/${encodeURIComponent(mod.slug)}`)
              }))
            }
          ]
        : [])
    ]
  })
)
useHead(() => ({
  meta: [{ name: 'keywords', content: homeKeywords.value }],
  link: [{ rel: 'canonical', href: homeCanonical }],
  script: [{ type: 'application/ld+json', innerHTML: homeJsonLd.value }]
}))
useSeoMeta({
  title: 'AttackBox — 理解原理，亲手验证。',
  description:
    '从教程到实践，在 AttackBox 学习网络安全。浏览学习模块，阅读公开章节，在终端和实验环境中亲手验证。',
  ogTitle: 'AttackBox — 理解原理，亲手验证。',
  ogDescription: '从教程到实践，探索网络安全。',
  ogUrl: homeCanonical,
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
  padding: 58px 0 80px;
  position: relative;
}

.modules:before {
  content: '';
  position: absolute;
  top: 0;
  left: 15%;
  right: 15%;
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(255, 255, 255, 0.14) 35%,
    rgba(249, 115, 22, 0.38) 50%,
    rgba(255, 255, 255, 0.14) 65%,
    transparent
  );
}

.section-head {
  text-align: center;
  margin-bottom: 40px;
}

.section-head h2 {
  font-size: 36px;
  font-weight: 700;
  letter-spacing: -1.4px;
  color: var(--ink);
}

.catalog-empty {
  display: grid;
  justify-items: center;
  gap: 18px;
  padding: 64px 24px;
  color: var(--muted);
  border: 1px solid var(--well-border);
  border-radius: 13px;
  background:
    radial-gradient(ellipse at 50% 100%, var(--brand-soft), transparent 70%), var(--surface-well);
  box-shadow: var(--well-shadow);
}

.catalog-empty :deep(svg) {
  color: var(--brand);
}

.catalog-empty p {
  font-size: 14px;
}

.catalog-link {
  display: flex;
  justify-content: center;
  margin-top: 34px;
}

.catalog-link a {
  color: var(--ink);
  font-size: 12.5px;
  font-weight: 600;
  padding: 11px 20px;
  border-radius: 8px;
  border: 1px solid var(--line-strong);
  background: var(--card-surface);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  text-decoration: none;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.catalog-link a:hover {
  transform: translateY(-2px);
  border-color: var(--brand-border);
  box-shadow: var(--card-hover-shadow);
}

@media (max-width: 760px) {
  .modules {
    padding: 36px 0 44px;
  }
  .section-head {
    margin-bottom: 28px;
  }
  .section-head h2 {
    font-size: 28px;
  }
}
</style>
