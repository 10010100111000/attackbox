<script setup lang="ts">
import { handleCardPointerMove } from '~/composables/useModuleRail'
import type { PublicModule } from '~/shared/module-catalog'

withDefaults(
  defineProps<{
    module: PublicModule
    featured?: boolean
    tone?: string
  }>(),
  { featured: false, tone: '#c1a381' }
)

const failedCover = ref(false)
const levelLabels = {
  beginner: '基础',
  intermediate: '进阶',
  advanced: '高阶'
} as const
</script>

<template>
  <NuxtLink
    :to="`/learn/modules/${module.slug}`"
    class="module-card"
    :class="{ 'featured-module': featured }"
    :data-module="module.slug"
    :aria-label="featured ? `浏览${module.title}模块` : undefined"
    :style="{ '--tone': tone, '--glow': tone }"
    @pointermove="handleCardPointerMove"
  >
    <div class="module-art" :class="{ 'cloud-art': featured }">
      <div class="card-topline">
        <span>{{ module.category?.name || '学习模块' }}</span>
        <span class="level">{{ levelLabels[module.difficulty] }}</span>
      </div>
      <img
        v-if="module.coverAssetId && !failedCover"
        :src="`/api/modules/${module.slug}/assets/${module.coverAssetId}`"
        alt=""
        loading="lazy"
        decoding="async"
        @error="failedCover = true"
      />
      <svg
        v-else-if="featured"
        class="diagram"
        viewBox="0 0 400 176"
        fill="none"
        aria-hidden="true"
        focusable="false"
      >
        <path
          d="M80 66L200 30L321 66L200 104Z M80 66V105L200 145L321 105V66 M200 104V145 M140 48L260 86 M140 85L260 48"
          stroke="currentColor"
          stroke-width=".8"
        />
        <path
          d="M140 48V87L200 107L260 87V48"
          stroke="currentColor"
          stroke-dasharray="3 5"
          opacity=".45"
        />
        <circle cx="200" cy="30" r="4" fill="currentColor" />
        <circle cx="80" cy="66" r="3" fill="currentColor" />
        <circle cx="321" cy="66" r="3" fill="currentColor" />
        <text
          x="170"
          y="88"
          fill="currentColor"
          stroke="none"
          font-size="10"
          font-family="monospace"
        >
          ATTACKBOX
        </text>
      </svg>
      <svg v-else class="diagram" viewBox="0 0 320 163" fill="none" aria-hidden="true">
        <path
          d="M70 108L160 55L250 108L160 143Z M70 80L160 27L250 80L160 115Z M70 80V108 M250 80V108 M160 115V143"
          stroke="currentColor"
          opacity=".6"
        />
        <circle cx="160" cy="55" r="4" fill="currentColor" />
        <path
          d="M160 27V55 M115 54L205 107 M115 107L205 54"
          stroke="currentColor"
          stroke-dasharray="3 5"
          opacity=".35"
        />
      </svg>
    </div>
    <div class="card-copy">
      <h3>{{ module.title }}</h3>
      <p v-if="module.summary">{{ module.summary }}</p>
      <div class="card-foot">
        <span
          >{{ module.chapterCount }} 章节<span v-if="module.durationMinutes">
            · {{ module.durationMinutes }} 分钟</span
          ></span
        >
      </div>
    </div>
  </NuxtLink>
</template>

<style scoped>
.module-card {
  --tone: #c1a381;
  --glow: #d3a275;
  position: relative;
  border: 1px solid var(--border-line, rgba(255, 255, 255, 0.07));
  background: var(--bg-card, #111217);
  border-radius: var(--radius-lg, 11px);
  overflow: hidden;
  isolation: isolate;
  display: block;
  transition:
    transform 0.3s,
    border-color 0.3s,
    box-shadow 0.3s;
}

.module-card:hover {
  transform: translateY(-4px);
  border-color: color-mix(in srgb, var(--tone) 50%, transparent);
  box-shadow: var(--card-glow, 0 15px 45px rgba(0, 0, 0, 0.4));
}

.module-art {
  height: 163px;
  position: relative;
  overflow: hidden;
  background: var(--bg-input, #121318);
}

.module-art:after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(0deg, var(--bg-card, #111217), transparent 75%);
}

.module-art:before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 2;
  background: radial-gradient(
    280px circle at var(--mx, 50%) var(--my, 40%),
    color-mix(in srgb, var(--glow) 14%, transparent),
    transparent 75%
  );
  opacity: 0;
  transition: opacity 0.35s;
}

.module-card:hover .module-art:before {
  opacity: 1;
}

.module-art img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.45;
  filter: saturate(0.35);
  transition:
    transform 0.7s,
    opacity 0.5s;
}

.module-card:hover img {
  transform: scale(1.07);
  opacity: 0.65;
}

.card-topline {
  position: absolute;
  top: 17px;
  left: 21px;
  right: 21px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font: 8px monospace;
  letter-spacing: 1.4px;
  color: #99909b;
  z-index: 3;
}

.card-topline > span:first-child {
  overflow-wrap: anywhere;
}

.level {
  font:
    9px / 1.5 -apple-system,
    sans-serif;
  letter-spacing: 0;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: var(--radius-sm, 4px);
  padding: 2px 6px;
  color: #d2b29c;
  background: rgba(23, 21, 26, 0.5);
}

.card-copy {
  padding: 0 23px 21px;
  position: relative;
  z-index: 3;
}

.card-copy h3 {
  font-size: 20px;
  letter-spacing: -0.5px;
  font-weight: 550;
  margin-bottom: 9px;
  overflow-wrap: anywhere;
}

.card-copy p {
  font-size: 11px;
  color: #8c8593;
  margin-bottom: 20px;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 9px;
  color: #7b7582;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  padding-top: 13px;
}

.cloud-art {
  background: radial-gradient(ellipse at 55% 30%, rgba(138, 91, 71, 0.2), transparent 66%), #121318;
}

.diagram {
  position: absolute;
  width: 100%;
  height: 100%;
  color: var(--tone);
  opacity: 0.65;
  transition: transform 0.7s;
}

.module-card:hover .diagram {
  transform: translateY(-5px);
}

.featured-module {
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  min-height: 310px;
  margin-bottom: 23px;
  background: linear-gradient(120deg, #18171c, var(--bg-card, #111217));
  border-color: rgba(195, 154, 123, 0.13);
}

.featured-module .module-art {
  height: 100%;
  min-height: 310px;
  background: radial-gradient(ellipse at 50% 50%, rgba(163, 107, 66, 0.19), transparent 68%);
}

.featured-module .module-art:after {
  background: linear-gradient(90deg, transparent 65%, #141419);
}

.featured-module .card-copy {
  padding: 46px;
  align-self: center;
}

.featured-module .card-copy h3 {
  font-size: 32px;
  letter-spacing: -1px;
  margin-bottom: 15px;
}

.featured-module .card-copy p {
  font-size: 12px;
  line-height: 1.9;
  max-width: 250px;
  margin-bottom: 31px;
}

.featured-module .card-foot {
  font-size: 10px;
}

.featured-module .diagram {
  transform: scale(0.93);
  opacity: 0.9;
}

.featured-module:hover .diagram {
  transform: scale(0.98) translateY(-3px);
}

@media (max-width: 1100px) {
  .featured-module .card-copy {
    padding: 32px;
  }
  .featured-module .card-copy h3 {
    font-size: 28px;
  }
  .card-copy {
    padding: 0 18px 18px;
  }
  .card-copy h3 {
    font-size: 19px;
  }
  .card-copy p {
    font-size: 10px;
  }
}

@media (max-width: 760px) {
  .featured-module {
    grid-template-columns: 1fr;
    min-height: 0;
    margin-bottom: 16px;
  }
  .featured-module .module-art {
    height: 200px;
    min-height: 0;
  }
  .featured-module .module-art:after {
    background: linear-gradient(0deg, #141419, transparent 70%);
  }
  .featured-module .card-copy {
    padding: 0 25px 24px;
  }
  .featured-module .card-copy h3 {
    font-size: 27px;
    margin-bottom: 11px;
  }
  .featured-module .card-copy p {
    max-width: 100%;
    font-size: 11px;
    margin-bottom: 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .module-card,
  .module-art img,
  .module-art:before,
  .diagram {
    transition: none;
  }
}
</style>
