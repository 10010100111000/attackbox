<script setup lang="ts">
import type { PublicModule } from '~/shared/module-catalog'
import { useModuleRail } from '~/composables/useModuleRail'
const props = defineProps<{ modules: PublicModule[] }>()
const railRef = ref<HTMLElement | null>(null)
const failedCovers = ref(new Set<string>())
const { handleCardPointerMove, measure } = useModuleRail(railRef)
watch(
  () => props.modules,
  () => nextTick(measure)
)
const tones = ['#d6b188', '#d3918f', '#87b8b1', '#a997c6', '#9ab0c7']
</script>

<template>
  <div
    ref="railRef"
    class="module-rail"
    tabindex="0"
    role="region"
    aria-label="更多学习模块，横向滚动浏览"
  >
    <NuxtLink
      v-for="(module, index) in modules"
      :key="module.id"
      :to="`/learn/modules/${module.slug}`"
      class="module-card"
      :data-module="module.slug"
      :style="{ '--tone': tones[index % tones.length], '--glow': tones[index % tones.length] }"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art">
        <div class="card-topline">
          <span>{{ module.category?.name || '学习模块' }}</span>
          <span class="level">{{
            { beginner: '基础', intermediate: '进阶', advanced: '高阶' }[module.difficulty]
          }}</span>
        </div>
        <img
          v-if="module.coverAssetId && !failedCovers.has(module.id)"
          :src="`/api/modules/${module.slug}/assets/${module.coverAssetId}`"
          alt=""
          loading="lazy"
          decoding="async"
          @error="failedCovers.add(module.id)"
        />
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
  </div>
</template>
