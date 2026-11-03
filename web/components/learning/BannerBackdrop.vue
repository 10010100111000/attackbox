<script setup lang="ts">
/** Stable across SSR, hydration and editor preview; Precision industrial telemetry & radar schematic. */
const props = defineProps<{ seed: string }>()
const identity = computed(() => {
  let hash = 2166136261
  for (const char of props.seed) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619) >>> 0
  return {
    color: ['#f97316', '#ea580c', '#38bdf8', '#10b981'][hash % 4],
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
        <!-- 精密等轴测基准坐标轴 -->
        <g stroke="currentColor" stroke-opacity="0.16" stroke-width="1">
          <line x1="560" y1="44" x2="1160" y2="44" stroke-dasharray="2 6" />
          <line x1="560" y1="120" x2="1160" y2="120" />
          <line x1="560" y1="196" x2="1160" y2="196" stroke-dasharray="2 6" />
          <line x1="780" y1="0" x2="780" y2="240" stroke-dasharray="3 5" />
          <line x1="980" y1="0" x2="980" y2="240" />
        </g>

        <!-- 光学雷达测距环 -->
        <g stroke="currentColor">
          <circle cx="980" cy="120" r="88" stroke-opacity="0.22" stroke-width="1.2" />
          <circle
            cx="980"
            cy="120"
            r="60"
            stroke-opacity="0.28"
            stroke-width="1"
            stroke-dasharray="5 4"
          />
          <circle cx="980" cy="120" r="28" stroke-opacity="0.36" stroke-width="1.2" />
          <circle cx="980" cy="120" r="4" fill="currentColor" fill-opacity="0.9" />
        </g>

        <!-- 动态变体几何拓扑模块 -->
        <polygon
          v-if="identity.motif !== 1"
          points="980,46 1044,120 980,194 916,120"
          stroke="currentColor"
          stroke-opacity="0.24"
          stroke-width="1"
        />
        <rect
          v-if="identity.motif !== 2"
          x="736"
          y="76"
          width="88"
          height="88"
          rx="8"
          stroke="currentColor"
          stroke-opacity="0.26"
          stroke-width="1.1"
        />

        <!-- 信号扫描矢量与关键信标锚点 -->
        <path
          d="M680 168 L780 120 L884 148 L980 120 L1088 68"
          stroke="currentColor"
          stroke-opacity="0.65"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <circle cx="780" cy="120" r="3.5" fill="currentColor" />
        <circle cx="884" cy="148" r="3" fill="currentColor" fill-opacity="0.75" />
        <circle cx="1088" cy="68" r="4" fill="currentColor" />
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
      ellipse 52% 95% at 84% 48%,
      color-mix(in srgb, var(--banner-accent) 14%, transparent),
      transparent 76%
    ),
    linear-gradient(110deg, var(--paper) 42%, color-mix(in srgb, var(--banner-accent) 5%, var(--soft)) 100%);
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
