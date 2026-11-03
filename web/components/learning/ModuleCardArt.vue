<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    index: number
    hasCover?: boolean
  }>(),
  { hasCover: false }
)

const artThemes = [
  {
    gradient:
      'radial-gradient(ellipse 78% 82% at 82% 22%, rgba(249, 115, 22, 0.1), transparent 70%)',
    accent: 'var(--brand)',
    type: 'stream'
  },
  {
    gradient:
      'radial-gradient(ellipse 78% 82% at 76% 24%, rgba(249, 115, 22, 0.085), transparent 70%)',
    accent: 'var(--brand)',
    type: 'constellation'
  },
  {
    gradient:
      'radial-gradient(ellipse 78% 82% at 84% 20%, rgba(249, 115, 22, 0.1), transparent 70%)',
    accent: 'var(--brand)',
    type: 'harmonic'
  },
  {
    gradient:
      'radial-gradient(ellipse 78% 82% at 78% 22%, rgba(249, 115, 22, 0.09), transparent 70%)',
    accent: 'var(--brand)',
    type: 'slipstream'
  },
  {
    gradient:
      'radial-gradient(ellipse 78% 82% at 80% 18%, rgba(249, 115, 22, 0.1), transparent 70%)',
    accent: 'var(--brand)',
    type: 'aurora'
  }
] as const

const theme = computed(() => artThemes[props.index % artThemes.length]!)
const formattedIndex = computed(() => String(props.index + 1).padStart(2, '0'))
const gradId = computed(() => `ab-card-art-${props.index}`)
</script>

<template>
  <div
    class="module-card-art-layer"
    :style="{ background: theme.gradient, '--art-accent': theme.accent }"
  >
    <svg
      v-if="!hasCover"
      class="art-svg"
      viewBox="0 0 320 118"
      preserveAspectRatio="xMidYMid slice"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <defs>
        <linearGradient :id="`${gradId}-accent`" x1="16" y1="0" x2="304" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="var(--art-accent)" stop-opacity="0" />
          <stop offset="28%" stop-color="var(--art-accent)" stop-opacity="0.4" />
          <stop offset="72%" stop-color="var(--art-accent)" stop-opacity="0.72" />
          <stop offset="100%" stop-color="var(--art-accent)" stop-opacity="0.05" />
        </linearGradient>
        <linearGradient :id="`${gradId}-ink`" x1="16" y1="0" x2="304" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="currentColor" stop-opacity="0" />
          <stop offset="32%" stop-color="currentColor" stop-opacity="0.16" />
          <stop offset="74%" stop-color="currentColor" stop-opacity="0.24" />
          <stop offset="100%" stop-color="currentColor" stop-opacity="0" />
        </linearGradient>
        <pattern :id="`${gradId}-dots`" x="0" y="0" width="18" height="18" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r="0.9" fill="currentColor" fill-opacity="0.08" />
        </pattern>
      </defs>

      <!-- 极细微粒点阵底纹 -->
      <rect x="64" y="0" width="256" height="118" :fill="`url(#${gradId}-dots)`" />

      <!-- 1. Stream: 双轨流光波束 + 悬浮微粒 -->
      <template v-if="theme.type === 'stream'">
        <path
          d="M 12 94 C 82 90, 128 32, 206 58 C 254 74, 284 34, 312 24"
          :stroke="`url(#${gradId}-accent)`"
          stroke-width="1.35"
          stroke-linecap="round"
        />
        <path
          d="M 18 68 C 88 38, 144 92, 218 62 C 264 42, 288 64, 312 52"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="1.1"
          stroke-linecap="round"
        />
        <path
          class="card-pulse"
          d="M 12 94 C 82 90, 128 32, 206 58 C 254 74, 284 34, 312 24"
          stroke="var(--art-accent)"
          stroke-width="1.9"
          stroke-linecap="round"
        />
        <circle class="card-particle c-1" cx="76" cy="72" r="1.4" fill="currentColor" fill-opacity="0.28" />
        <circle class="card-particle c-2" cx="126" cy="52" r="2.2" fill="var(--art-accent)" fill-opacity="0.85" />
        <circle class="card-particle c-2" cx="126" cy="52" r="5.5" fill="var(--art-accent)" fill-opacity="0.14" />
        <circle class="card-particle c-3" cx="168" cy="74" r="1.3" fill="currentColor" fill-opacity="0.24" />
        <circle class="card-particle c-1" cx="206" cy="58" r="2.4" fill="var(--art-accent)" />
        <circle class="card-particle c-1" cx="206" cy="58" r="6" fill="var(--art-accent)" fill-opacity="0.15" />
        <circle class="card-particle c-2" cx="248" cy="38" r="1.5" fill="var(--art-accent)" fill-opacity="0.55" />
      </template>

      <!-- 2. Constellation: 脉冲星群轨迹 -->
      <template v-else-if="theme.type === 'constellation'">
        <path
          d="M 16 86 C 92 78, 148 36, 224 44 C 266 48, 288 30, 312 22"
          :stroke="`url(#${gradId}-accent)`"
          stroke-width="1.3"
          stroke-linecap="round"
        />
        <path
          d="M 68 74 L 122 46 L 178 64 L 224 44 L 268 62"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="0.95"
          stroke-dasharray="2 4"
        />
        <path
          class="card-pulse"
          d="M 16 86 C 92 78, 148 36, 224 44 C 266 48, 288 30, 312 22"
          stroke="var(--art-accent)"
          stroke-width="1.85"
          stroke-linecap="round"
        />
        <circle class="card-particle c-1" cx="68" cy="74" r="1.5" fill="currentColor" fill-opacity="0.3" />
        <circle class="card-particle c-2" cx="122" cy="46" r="2.2" fill="var(--art-accent)" fill-opacity="0.85" />
        <circle class="card-particle c-3" cx="152" cy="30" r="1.2" fill="currentColor" fill-opacity="0.22" />
        <circle class="card-particle c-1" cx="178" cy="64" r="1.8" fill="currentColor" fill-opacity="0.35" />
        <circle class="card-particle c-2" cx="224" cy="44" r="2.5" fill="var(--art-accent)" />
        <circle class="card-particle c-2" cx="224" cy="44" r="6.5" fill="var(--art-accent)" fill-opacity="0.16" />
        <circle class="card-particle c-3" cx="268" cy="62" r="1.5" fill="var(--art-accent)" fill-opacity="0.5" />
      </template>

      <!-- 3. Harmonic: 多重谐波信号包络 -->
      <template v-else-if="theme.type === 'harmonic'">
        <path
          d="M 10 82 C 66 44, 118 98, 178 54 C 228 18, 266 66, 312 34"
          :stroke="`url(#${gradId}-accent)`"
          stroke-width="1.35"
          stroke-linecap="round"
        />
        <path
          d="M 10 62 C 64 28, 122 78, 182 38 C 232 10, 268 48, 312 20"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="1.05"
          stroke-linecap="round"
        />
        <path
          d="M 10 98 C 72 68, 124 108, 184 72 C 234 42, 272 82, 312 54"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="0.85"
          stroke-dasharray="2 4"
          stroke-linecap="round"
        />
        <path
          class="card-pulse"
          d="M 10 82 C 66 44, 118 98, 178 54 C 228 18, 266 66, 312 34"
          stroke="var(--art-accent)"
          stroke-width="1.9"
          stroke-linecap="round"
        />
        <circle class="card-particle c-2" cx="92" cy="68" r="1.4" fill="currentColor" fill-opacity="0.28" />
        <circle class="card-particle c-1" cx="146" cy="42" r="1.6" fill="var(--art-accent)" fill-opacity="0.6" />
        <circle class="card-particle c-3" cx="178" cy="54" r="2.4" fill="var(--art-accent)" />
        <circle class="card-particle c-3" cx="178" cy="54" r="6" fill="var(--art-accent)" fill-opacity="0.15" />
        <circle class="card-particle c-1" cx="222" cy="36" r="1.5" fill="currentColor" fill-opacity="0.28" />
        <circle class="card-particle c-2" cx="256" cy="48" r="2" fill="var(--art-accent)" fill-opacity="0.75" />
      </template>

      <!-- 4. Slipstream: 跃迁引力流线场 -->
      <template v-else-if="theme.type === 'slipstream'">
        <path
          d="M 20 102 C 104 96, 168 68, 242 26"
          :stroke="`url(#${gradId}-accent)`"
          stroke-width="1.35"
          stroke-linecap="round"
        />
        <path
          d="M 48 108 C 126 98, 188 74, 268 36"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="1.05"
          stroke-linecap="round"
        />
        <path
          d="M 8 78 C 88 74, 152 52, 222 18"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="0.9"
          stroke-dasharray="3 5"
          stroke-linecap="round"
        />
        <path
          class="card-pulse"
          d="M 20 102 C 104 96, 168 68, 242 26"
          stroke="var(--art-accent)"
          stroke-width="1.9"
          stroke-linecap="round"
        />
        <circle class="card-particle c-1" cx="96" cy="88" r="1.4" fill="currentColor" fill-opacity="0.26" />
        <circle class="card-particle c-2" cx="148" cy="72" r="2" fill="var(--art-accent)" fill-opacity="0.75" />
        <circle class="card-particle c-3" cx="196" cy="50" r="2.4" fill="var(--art-accent)" />
        <circle class="card-particle c-3" cx="196" cy="50" r="6" fill="var(--art-accent)" fill-opacity="0.15" />
        <circle class="card-particle c-1" cx="236" cy="64" r="1.5" fill="currentColor" fill-opacity="0.28" />
      </template>

      <!-- 5. Aurora: 极光波纹与微粒涟漪 -->
      <template v-else>
        <path
          d="M 12 76 C 78 96, 142 28, 214 54 C 258 70, 286 44, 312 38"
          :stroke="`url(#${gradId}-accent)`"
          stroke-width="1.35"
          stroke-linecap="round"
        />
        <path
          d="M 12 54 C 82 74, 146 18, 218 40 C 262 54, 288 32, 312 24"
          :stroke="`url(#${gradId}-ink)`"
          stroke-width="1.05"
          stroke-linecap="round"
        />
        <path
          class="card-pulse"
          d="M 12 76 C 78 96, 142 28, 214 54 C 258 70, 286 44, 312 38"
          stroke="var(--art-accent)"
          stroke-width="1.9"
          stroke-linecap="round"
        />
        <circle class="card-particle c-2" cx="84" cy="64" r="1.5" fill="currentColor" fill-opacity="0.26" />
        <circle class="card-particle c-1" cx="138" cy="48" r="2.2" fill="var(--art-accent)" fill-opacity="0.82" />
        <circle class="card-particle c-3" cx="182" cy="36" r="1.4" fill="currentColor" fill-opacity="0.3" />
        <circle class="card-particle c-2" cx="214" cy="54" r="2.4" fill="var(--art-accent)" />
        <circle class="card-particle c-2" cx="214" cy="54" r="6" fill="var(--art-accent)" fill-opacity="0.15" />
      </template>
    </svg>

    <div class="art-index-pill">
      <span class="pill-dot" />
      <span>{{ formattedIndex }}</span>
    </div>
  </div>
</template>

<style scoped>
.module-card-art-layer {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  padding: 14px 16px;
  pointer-events: none;
}

.art-svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  color: var(--ink);
}

.card-pulse {
  stroke-dasharray: 42 280;
  stroke-dashoffset: 322;
  animation: card-stream 7s linear infinite;
}

.card-particle {
  animation: card-float 4.8s ease-in-out infinite alternate;
}

.c-2 {
  animation-duration: 6.2s;
  animation-delay: -1.8s;
}

.c-3 {
  animation-duration: 5.5s;
  animation-delay: -3.2s;
}

@keyframes card-stream {
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes card-float {
  0% {
    transform: translateY(-2px);
    opacity: 0.6;
  }
  100% {
    transform: translateY(2px);
    opacity: 1;
  }
}

.art-index-pill {
  position: relative;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3.5px 9px;
  border-radius: 6px;
  border: 1px solid var(--line-strong, rgba(255, 255, 255, 0.14));
  background: color-mix(in srgb, var(--paper) 88%, transparent);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  box-shadow: var(--surface-highlight);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: var(--ink);
}

.pill-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--art-accent, var(--brand));
}

@media (prefers-reduced-motion: reduce) {
  .card-pulse,
  .card-particle {
    animation: none;
  }
}
</style>
