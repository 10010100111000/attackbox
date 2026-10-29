<script setup lang="ts">
import ModuleCard from './ModuleCard.vue'
import { useModuleRail } from '~/composables/useModuleRail'
import type { PublicModule } from '~/shared/module-catalog'

const props = defineProps<{ modules: PublicModule[] }>()
const railRef = ref<HTMLElement | null>(null)
const { measure } = useModuleRail(railRef)
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
    aria-label="更多学习模块,横向滚动浏览"
  >
    <ModuleCard
      v-for="(module, index) in modules"
      :key="module.id"
      :module="module"
      :tone="tones[index % tones.length]"
    />
  </div>
</template>

<style scoped>
.module-rail {
  display: flex;
  gap: 20px;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 6px 1px 15px;
  scrollbar-width: none;
  overscroll-behavior-x: contain;
  overflow-anchor: none;
}

.module-rail::-webkit-scrollbar {
  display: none;
}

.module-rail > :deep(.module-card) {
  flex: 0 0 31%;
}

.module-rail:focus-visible {
  outline: 1px solid rgba(195, 154, 123, 0.4);
  outline-offset: 6px;
}

@media (max-width: 1100px) {
  .module-rail > :deep(.module-card) {
    flex-basis: 38%;
  }
}

@media (max-width: 760px) {
  .module-rail {
    gap: 14px;
    margin-right: -20px;
    padding-right: 20px;
    padding-bottom: 17px;
  }
  .module-rail > :deep(.module-card) {
    flex-basis: 82%;
  }
  .module-rail :deep(.module-art) {
    height: 157px;
  }
  .module-rail :deep(.card-copy) {
    padding: 0 21px 20px;
  }
  .module-rail :deep(.card-copy h3) {
    font-size: 20px;
  }
}
</style>
