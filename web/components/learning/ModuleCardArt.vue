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
      'radial-gradient(ellipse 80% 85% at 82% 20%, rgba(249, 115, 22, 0.11), transparent 68%)',
    accent: 'var(--brand)',
    type: 'radar'
  },
  {
    gradient:
      'radial-gradient(ellipse 80% 85% at 82% 20%, rgba(249, 115, 22, 0.08), transparent 68%)',
    accent: 'var(--brand)',
    type: 'grid'
  },
  {
    gradient:
      'radial-gradient(ellipse 80% 85% at 82% 20%, rgba(249, 115, 22, 0.1), transparent 68%)',
    accent: 'var(--brand)',
    type: 'nodes'
  },
  {
    gradient:
      'radial-gradient(ellipse 80% 85% at 82% 20%, rgba(249, 115, 22, 0.09), transparent 68%)',
    accent: 'var(--brand)',
    type: 'waves'
  },
  {
    gradient:
      'radial-gradient(ellipse 80% 85% at 82% 20%, rgba(249, 115, 22, 0.11), transparent 68%)',
    accent: 'var(--brand)',
    type: 'rings'
  }
] as const

const theme = computed(() => artThemes[props.index % artThemes.length]!)
const formattedIndex = computed(() => String(props.index + 1).padStart(2, '0'))
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
      <!-- 全幅底层基准网格与技术刻度 -->
      <g stroke="currentColor" stroke-opacity="0.09" stroke-width="1">
        <line x1="0" y1="30" x2="320" y2="30" stroke-dasharray="2 5" />
        <line x1="0" y1="62" x2="320" y2="62" />
        <line x1="0" y1="94" x2="320" y2="94" stroke-dasharray="2 5" />
        <line x1="64" y1="0" x2="64" y2="118" stroke-dasharray="2 5" />
        <line x1="160" y1="0" x2="160" y2="118" />
        <line x1="248" y1="0" x2="248" y2="118" stroke-dasharray="2 5" />
      </g>

      <!-- 拓扑节点网络 -->
      <template v-if="theme.type === 'nodes'">
        <path
          d="M42 82 L108 42 L176 74 L236 32 L288 58"
          stroke="currentColor"
          stroke-opacity="0.28"
          stroke-width="1.3"
        />
        <path
          d="M108 42 L176 26 L236 32"
          stroke="var(--art-accent)"
          stroke-opacity="0.55"
          stroke-width="1.3"
          stroke-dasharray="3 3"
        />
        <circle cx="42" cy="82" r="3" fill="currentColor" fill-opacity="0.4" />
        <circle cx="108" cy="42" r="4" fill="var(--art-accent)" fill-opacity="0.85" />
        <circle cx="176" cy="74" r="3.5" fill="currentColor" fill-opacity="0.5" />
        <circle cx="236" cy="32" r="4.5" fill="var(--art-accent)" />
        <circle
          cx="236"
          cy="32"
          r="11"
          stroke="var(--art-accent)"
          stroke-opacity="0.45"
          stroke-width="1.2"
        />
      </template>

      <!-- 精密雷达同心刻度 -->
      <template v-else-if="theme.type === 'radar' || theme.type === 'rings'">
        <circle
          cx="248"
          cy="86"
          r="26"
          stroke="var(--art-accent)"
          stroke-opacity="0.48"
          stroke-width="1.3"
        />
        <circle
          cx="248"
          cy="86"
          r="52"
          stroke="currentColor"
          stroke-opacity="0.28"
          stroke-width="1.2"
          stroke-dasharray="4 4"
        />
        <circle
          cx="248"
          cy="86"
          r="82"
          stroke="currentColor"
          stroke-opacity="0.16"
          stroke-width="1.1"
        />
        <circle
          cx="248"
          cy="86"
          r="116"
          stroke="currentColor"
          stroke-opacity="0.1"
          stroke-width="1"
        />
        <line
          x1="248"
          y1="86"
          x2="176"
          y2="28"
          stroke="var(--art-accent)"
          stroke-opacity="0.78"
          stroke-width="1.6"
        />
        <circle cx="176" cy="28" r="4" fill="var(--art-accent)" />
        <circle cx="248" cy="86" r="3.5" fill="var(--art-accent)" />
      </template>

      <!-- 工业坐标矩阵 -->
      <template v-else-if="theme.type === 'grid'">
        <rect
          x="132"
          y="24"
          width="116"
          height="70"
          rx="6"
          stroke="currentColor"
          stroke-opacity="0.24"
          stroke-width="1.2"
        />
        <rect
          x="166"
          y="42"
          width="48"
          height="34"
          rx="4"
          stroke="var(--art-accent)"
          stroke-opacity="0.72"
          stroke-width="1.4"
        />
        <line
          x1="88"
          y1="59"
          x2="276"
          y2="59"
          stroke="var(--art-accent)"
          stroke-opacity="0.45"
          stroke-width="1.2"
          stroke-dasharray="3 3"
        />
        <circle cx="190" cy="59" r="3.5" fill="var(--art-accent)" />
      </template>

      <!-- 信号频谱波形 -->
      <template v-else>
        <path
          d="M16 86 C68 52, 112 96, 168 54 C212 22, 248 68, 304 34"
          stroke="var(--art-accent)"
          stroke-opacity="0.72"
          stroke-width="1.7"
        />
        <path
          d="M16 64 C64 34, 118 74, 174 38 C218 12, 254 48, 304 20"
          stroke="currentColor"
          stroke-opacity="0.28"
          stroke-width="1.2"
          stroke-dasharray="3 3"
        />
        <circle cx="168" cy="54" r="3.5" fill="var(--art-accent)" />
        <circle cx="238" cy="44" r="3" fill="currentColor" fill-opacity="0.45" />
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
  transition: transform 0.35s cubic-bezier(0.22, 1, 0.36, 1);
}

:global(.card:hover) .art-svg {
  transform: scale(1.04);
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
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;
}

.pill-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--art-accent, var(--brand));
}

:global(.card:hover) .art-index-pill {
  transform: translateY(-1px);
  border-color: var(--brand-border);
  color: var(--ink);
}

@media (prefers-reduced-motion: reduce) {
  .art-svg,
  .art-index-pill {
    transition: none;
  }
  :global(.card:hover) .art-svg,
  :global(.card:hover) .art-index-pill {
    transform: none;
  }
}
</style>
