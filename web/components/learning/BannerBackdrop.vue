<script setup lang="ts">
/** Stable across SSR, hydration and editor preview; Precision industrial telemetry & radar schematic. */
const props = defineProps<{ seed: string }>()
const identity = computed(() => {
  let hash = 2166136261
  for (const char of props.seed) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619) >>> 0
  return {
    color: ['#ea580c', '#f97316', '#ea580c', '#f97316'][hash % 4],
    motif: (hash >>> 8) % 3
  }
})
</script>

<template>
  <div class="banner-backdrop" :style="{ '--banner-accent': identity.color }" aria-hidden="true">
    <svg viewBox="0 0 1200 240" preserveAspectRatio="xMidYMid slice" fill="none">
      <defs>
        <linearGradient id="ab-banner-fade" x1="460" y1="0" x2="720" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="1" />
        </linearGradient>
        <mask id="ab-banner-mask">
          <rect x="460" y="0" width="740" height="240" fill="url(#ab-banner-fade)" />
        </mask>
      </defs>

      <g mask="url(#ab-banner-mask)">
        <!-- 精密等轴测基准坐标轴 (结构锌灰) -->
        <g stroke="var(--ink)" stroke-opacity="0.08" stroke-width="1">
          <line x1="520" y1="44" x2="1180" y2="44" stroke-dasharray="2 6" />
          <line x1="520" y1="120" x2="1180" y2="120" />
          <line x1="520" y1="196" x2="1180" y2="196" stroke-dasharray="2 6" />
          <line x1="780" y1="0" x2="780" y2="240" stroke-dasharray="3 5" />
          <line x1="980" y1="0" x2="980" y2="240" />
        </g>

        <!-- 光学雷达测距环 (外环结构锌灰 + 内环信号橙锁定) -->
        <g>
          <circle cx="980" cy="120" r="96" stroke="var(--ink)" stroke-opacity="0.12" stroke-width="1.2" />
          <circle
            cx="980"
            cy="120"
            r="64"
            stroke="var(--ink)"
            stroke-opacity="0.18"
            stroke-width="1.1"
            stroke-dasharray="5 4"
          />
          <circle cx="980" cy="120" r="32" stroke="currentColor" stroke-opacity="0.55" stroke-width="1.4" />
          <circle cx="980" cy="120" r="4.5" fill="currentColor" />
        </g>

        <!-- 动态变体几何拓扑模块 -->
        <polygon
          v-if="identity.motif !== 1"
          points="980,42 1048,120 980,198 912,120"
          stroke="var(--ink)"
          stroke-opacity="0.14"
          stroke-width="1.1"
        />
        <rect
          v-if="identity.motif !== 2"
          x="732"
          y="72"
          width="96"
          height="96"
          rx="8"
          stroke="var(--ink)"
          stroke-opacity="0.15"
          stroke-width="1.2"
        />

        <!-- 信号扫描矢量与关键信标锚点 -->
        <path
          d="M660 172 L780 120 L884 150 L980 120 L1096 64"
          stroke="currentColor"
          stroke-opacity="0.88"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <circle cx="780" cy="120" r="4" fill="currentColor" />
        <circle cx="884" cy="150" r="3.5" fill="var(--ink)" fill-opacity="0.45" />
        <circle cx="1096" cy="64" r="4.5" fill="currentColor" />
      </g>
    </svg>
  </div>
</template>

<style scoped>
.banner-backdrop {
  position: absolute;
  inset: 0;
  z-index: -2;
  pointer-events: none;
  color: var(--banner-accent);
  background:
    radial-gradient(
      ellipse 46% 90% at 84% 50%,
      color-mix(in srgb, var(--banner-accent) 10%, transparent),
      transparent 74%
    ),
    linear-gradient(110deg, #ffffff 48%, #fafaf9 100%);
}

:global([data-theme='midnight']) .banner-backdrop {
  background:
    radial-gradient(
      ellipse 52% 95% at 84% 48%,
      color-mix(in srgb, var(--banner-accent) 22%, transparent),
      transparent 76%
    ),
    linear-gradient(110deg, var(--paper) 42%, color-mix(in srgb, var(--banner-accent) 9%, #16161b) 100%);
}

svg {
  width: 100%;
  height: 100%;
  position: absolute;
  inset: 0;
}

@media (max-width: 640px) {
  svg {
    opacity: 0.35;
  }
}
</style>
