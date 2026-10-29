<script setup lang="ts">
import { computed } from 'vue'
import { ArrowLeft, BookOpen, Clock3, CheckCircle2, ArrowRight } from 'lucide-vue-next'
import type { LearningRoute } from '~/shared/routes'
import { useAuth } from '~/composables/useAuth'

definePageMeta({ layout: 'learning', key: route => route.fullPath })

const { currentUser } = useAuth()
const learner = computed(() => currentUser.value?.id || '')
const routeId = String(useRoute().params.id)

const { data, error } = await useFetch<{ data: LearningRoute }>(`/api/routes/${routeId}`, {
  key: `learning:route:${routeId}`,
  watch: [learner]
})

const route = computed(() => data.value?.data)
const next = computed(
  () =>
    route.value?.modules?.find(m => m.completedChapters < m.chapterCount) ||
    route.value?.modules?.[0]
)

useHead(() => ({
  title: route.value ? `${route.value.title} — AttackBox` : '学习路线 — AttackBox',
  meta: [{ name: 'description', content: route.value?.summary || '按推荐顺序学习相关模块。' }]
}))
</script>

<template>
  <main class="route-page">
    <NuxtLink to="/learn?tab=routes" class="back">
      <ArrowLeft :size="14" aria-hidden="true" />
      <span>学习路线</span>
    </NuxtLink>

    <p v-if="error" role="alert">路线不存在或暂不可用。</p>

    <template v-else-if="route">
      <header class="route-header">
        <div class="eyebrow">学习路线</div>
        <h1>{{ route.title }}</h1>
        <p>{{ route.summary }}</p>

        <div class="meta">
          <span>
            <BookOpen :size="15" aria-hidden="true" />
            {{ route.moduleCount }} 个模块 · {{ route.chapterCount }} 章
          </span>
          <span v-if="route.durationMinutes">
            <Clock3 :size="15" aria-hidden="true" />
            已配置时长 {{ route.durationMinutes }} 分钟
          </span>
          <span v-if="currentUser">
            已完成 {{ route.completedChapters }} / {{ route.chapterCount }} 章
          </span>
        </div>

        <NuxtLink v-if="next" :to="`/learn/modules/${next.slug}`" class="cta">
          <span>
            {{
              route.chapterCount > 0 && route.completedChapters === route.chapterCount
                ? '回顾路线'
                : route.completedChapters
                  ? '继续学习'
                  : '开始学习'
            }}
          </span>
          <ArrowRight :size="16" aria-hidden="true" />
        </NuxtLink>
      </header>

      <section aria-label="路线模块">
        <NuxtLink
          v-for="(item, index) in route.modules"
          :key="item.id"
          :to="`/learn/modules/${item.slug}`"
          class="route-module"
        >
          <span class="number">
            <CheckCircle2
              v-if="
                currentUser && item.chapterCount > 0 && item.completedChapters === item.chapterCount
              "
              :size="22"
              aria-hidden="true"
            />
            <template v-else>{{ String(index + 1).padStart(2, '0') }}</template>
          </span>
          <div>
            <h2>{{ item.title }}</h2>
            <p>{{ item.summary }}</p>
            <small>
              {{ item.chapterCount }} 章
              <span v-if="currentUser"> · 已完成 {{ item.completedChapters }} 章</span>
            </small>
          </div>
          <ArrowRight :size="18" aria-hidden="true" />
        </NuxtLink>
      </section>
    </template>
  </main>
</template>

<style scoped>
.route-page {
  width: 100%;
  max-width: 1000px;
  margin: 0 auto;
  padding: 42px 28px 80px;
  color: var(--ink);
  box-sizing: border-box;
}

a {
  text-decoration: none;
  color: inherit;
}

.back {
  display: inline-flex;
  gap: 7px;
  font-size: 12px;
  align-items: center;
  color: var(--muted);
}

.back:hover {
  color: var(--ink);
}

.route-header {
  padding: 36px 0;
}

.eyebrow {
  font-size: 12px;
  color: var(--green);
  margin-bottom: 12px;
}

h1 {
  font-size: clamp(28px, 3.2vw, 36px);
  margin: 0 0 18px;
}

.route-header p {
  max-width: 700px;
  font-size: 15px;
  line-height: 1.8;
  color: var(--muted);
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 24px;
  color: var(--muted);
  font-size: 12px;
  margin: 24px 0;
}

.meta span,
.cta {
  display: inline-flex;
  gap: 8px;
  align-items: center;
}

.cta {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  box-shadow: var(--btn-primary-shadow);
  border-radius: 8px;
  padding: 12px 22px;
  font-size: 13px;
  font-weight: 600;
  transition:
    transform 0.16s ease,
    background 0.16s ease;
}

.cta:hover {
  transform: translateY(-1px);
  background: var(--btn-primary-hover);
}

.route-module {
  display: flex;
  align-items: center;
  gap: 24px;
  border-top: 1px solid var(--line);
  padding: 28px 8px;
  transition: background 0.15s ease;
}

.route-module:hover {
  background: var(--soft);
}

.route-module > div {
  flex: 1;
  min-width: 0;
}

.number {
  width: 42px;
  color: var(--green);
  font:
    22px ui-monospace,
    monospace;
  flex-shrink: 0;
}

h2 {
  font-size: 19px;
  margin: 0 0 10px;
}

.route-module p {
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted);
  margin: 0 0 10px;
}

small {
  color: var(--muted);
  font-size: 12px;
}
</style>
