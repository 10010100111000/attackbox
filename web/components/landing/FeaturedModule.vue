<script setup lang="ts">
import { handleCardPointerMove } from '~/composables/useModuleRail'
import type { PublicModule } from '~/shared/module-catalog'
defineProps<{ module: PublicModule }>()
const failedCover = ref(false)
</script>

<template>
  <NuxtLink
    :to="`/learn/modules/${module.slug}`"
    class="module-card featured-module"
    :data-module="module.slug"
    style="--tone: #d8a57b; --glow: #d8a57b"
    :aria-label="`浏览${module.title}模块`"
    @pointermove="handleCardPointerMove"
  >
    <div class="module-art cloud-art">
      <div class="card-topline">
        <span>{{ module.category?.name || '学习模块' }}</span>
        <span class="level">{{
          { beginner: '基础', intermediate: '进阶', advanced: '高阶' }[module.difficulty]
        }}</span>
      </div>
      <img
        v-if="module.coverAssetId && !failedCover"
        :src="`/api/modules/${module.slug}/assets/${module.coverAssetId}`"
        alt=""
        loading="lazy"
        @error="failedCover = true"
      />
      <svg
        v-else
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
