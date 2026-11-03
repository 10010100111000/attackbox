<script setup lang="ts">
/** Stable across SSR, hydration and editor preview; Dynamic flowing signal lines & micro-particle field. */
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
        <linearGradient id="ab-banner-fade" x1="420" y1="0" x2="720" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="1" />
        </linearGradient>
        <mask id="ab-banner-mask">
          <rect x="420" y="0" width="780" height="240" fill="url(#ab-banner-fade)" />
        </mask>

        <linearGradient id="ab-flow-accent" x1="480" y1="0" x2="1180" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="currentColor" stop-opacity="0" />
          <stop offset="28%" stop-color="currentColor" stop-opacity="0.42" />
          <stop offset="68%" stop-color="currentColor" stop-opacity="0.72" />
          <stop offset="100%" stop-color="currentColor" stop-opacity="0.08" />
        </linearGradient>

        <linearGradient id="ab-flow-ink" x1="500" y1="0" x2="1180" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="var(--ink)" stop-opacity="0" />
          <stop offset="35%" stop-color="var(--ink)" stop-opacity="0.16" />
          <stop offset="75%" stop-color="var(--ink)" stop-opacity="0.22" />
          <stop offset="100%" stop-color="var(--ink)" stop-opacity="0" />
        </linearGradient>

        <pattern id="ab-banner-dots" x="0" y="0" width="22" height="22" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r="1" fill="var(--ink)" fill-opacity="0.1" />
        </pattern>
      </defs>

      <g mask="url(#ab-banner-mask)">
        <!-- 微米级点阵背景场 -->
        <rect x="520" y="0" width="680" height="240" fill="url(#ab-banner-dots)" />

        <!-- 多层柔顺流光谐波线束 -->
        <path
          d="M 480 196 C 640 188, 740 66, 920 108 C 1045 136, 1115 54, 1200 38"
          stroke="url(#ab-flow-accent)"
          stroke-width="1.35"
          stroke-linecap="round"
        />
        <path
          d="M 500 144 C 668 96, 788 182, 956 126 C 1068 88, 1134 122, 1200 94"
          stroke="url(#ab-flow-ink)"
          stroke-width="1.15"
          stroke-linecap="round"
        />
        <path
          v-if="identity.motif !== 1"
          d="M 530 214 C 702 162, 826 198, 988 148 C 1088 118, 1144 76, 1200 66"
          stroke="url(#ab-flow-ink)"
          stroke-width="0.9"
          stroke-dasharray="3 5"
          stroke-linecap="round"
        />
        <path
          v-if="identity.motif !== 2"
          d="M 550 72 C 712 134, 836 46, 1004 86 C 1098 108, 1148 146, 1200 156"
          stroke="url(#ab-flow-accent)"
          stroke-opacity="0.55"
          stroke-width="0.95"
          stroke-linecap="round"
        />

        <!-- 动态流光脉冲轨迹 -->
        <path
          class="flow-pulse"
          d="M 480 196 C 640 188, 740 66, 920 108 C 1045 136, 1115 54, 1200 38"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        />
        <path
          class="flow-pulse flow-pulse-delayed"
          d="M 500 144 C 668 96, 788 182, 956 126 C 1068 88, 1134 122, 1200 94"
          stroke="currentColor"
          stroke-opacity="0.7"
          stroke-width="1.6"
          stroke-linecap="round"
        />

        <!-- 悬浮微粒子星尘场 -->
        <g class="particle-group">
          <circle class="particle-drift p-1" cx="648" cy="148" r="1.8" fill="currentColor" fill-opacity="0.55" />
          <circle class="particle-drift p-2" cx="716" cy="96" r="1.4" fill="var(--ink)" fill-opacity="0.28" />
          <circle class="particle-drift p-3" cx="764" cy="126" r="2.6" fill="currentColor" fill-opacity="0.88" />
          <circle class="particle-drift p-3" cx="764" cy="126" r="7" fill="currentColor" fill-opacity="0.14" />

          <circle class="particle-drift p-2" cx="828" cy="78" r="1.5" fill=" currentColor" fill-opacity="0.45" />
          <circle class="particle-drift p-1" cx="868" cy="158" r="1.6" fill="var(--ink)" fill-opacity="0.25" />
          <circle class="particle-drift p-3" cx="920" cy="108" r="2.8" fill="currentColor" />
          <circle class="particle-drift p-3" cx="920" cy="108" r="8" fill="currentColor" fill-opacity="0.16" />

          <circle class="particle-drift p-1" cx="976" cy="68" r="1.5" fill="var(--ink)" fill-opacity="0.3" />
          <circle class="particle-drift p-2" cx="1012" cy="142" r="2" fill="currentColor" fill-opacity="0.6" />
          <circle class="particle-drift p-1" cx="1064" cy="98" r="1.4" fill="var(--ink)" fill-opacity="0.26" />
          <circle class="particle-drift p-2" cx="1116" cy="62" r="2.6" fill="currentColor" fill-opacity="0.92" />
          <circle class="particle-drift p-2" cx="1116" cy="62" r="6.5" fill="currentColor" fill-opacity="0.15" />
          <circle class="particle-drift p-3" cx="1154" cy="118" r="1.6" fill="currentColor" fill-opacity="0.5" />
        </g>
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
      ellipse 48% 92% at 84% 48%,
      color-mix(in srgb, var(--banner-accent) 13%, transparent),
      transparent 74%
    ),
    linear-gradient(
      110deg,
      var(--paper) 46%,
      color-mix(in srgb, var(--banner-accent) 6%, var(--paper)) 100%
    );
}

svg {
  width: 100%;
  height: 100%;
  position: absolute;
  inset: 0;
}

.flow-pulse {
  stroke-dasharray: 68 540;
  stroke-dashoffset: 608;
  animation: banner-stream 8.5s linear infinite;
}

.flow-pulse-delayed {
  stroke-dasharray: 48 560;
  stroke-dashoffset: 608;
  animation-duration: 10.5s;
  animation-delay: -3.5s;
}

.particle-drift {
  animation: banner-float 5.5s ease-in-out infinite alternate;
}

.p-2 {
  animation-duration: 7s;
  animation-delay: -2s;
}

.p-3 {
  animation-duration: 6.2s;
  animation-delay: -4s;
}

@keyframes banner-stream {
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes banner-float {
  0% {
    transform: translateY(-2.5px);
    opacity: 0.65;
  }
  100% {
    transform: translateY(2.5px);
    opacity: 1;
  }
}

@media (max-width: 640px) {
  svg {
    opacity: 0.35;
  }
}

@media (prefers-reduced-motion: reduce) {
  .flow-pulse,
  .particle-drift {
    animation: none;
  }
}
</style>
