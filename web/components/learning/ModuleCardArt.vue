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
      'radial-gradient(ellipse 90% 85% at 78% 18%, rgba(249, 115, 22, 0.18), transparent 68%), linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent)',
    accent: '#f97316',
    type: 'radar'
  },
  {
    gradient:
      'radial-gradient(ellipse 90% 85% at 78% 18%, rgba(228, 228, 231, 0.13), transparent 68%), linear-gradient(180deg, rgba(255, 255, 255, 0.025), transparent)',
    accent: '#d4d4d8',
    type: 'grid'
  },
  {
    gradient:
      'radial-gradient(ellipse 90% 85% at 78% 18%, rgba(56, 189, 248, 0.14), transparent 68%), linear-gradient(180deg, rgba(255, 255, 255, 0.025), transparent)',
    accent: '#38bdf8',
    type: 'nodes'
  },
  {
    gradient:
      'radial-gradient(ellipse 90% 85% at 78% 18%, rgba(52, 211, 153, 0.14), transparent 68%), linear-gradient(180deg, rgba(255, 255, 255, 0.025), transparent)',
    accent: '#34d399',
    type: 'waves'
  },
  {
    gradient:
      'radial-gradient(ellipse 90% 85% at 78% 18%, rgba(251, 146, 60, 0.15), transparent 68%), linear-gradient(180deg, rgba(255, 255, 255, 0.025), transparent)',
    accent: '#fb923c',
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
      viewBox="0 0 120 68"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <!-- 拓扑节点网络 -->
      <template v-if="theme.type === 'nodes'">
        <circle cx="24" cy="36" r="2.5" fill="var(--art-accent)" fill-opacity="0.7" />
        <circle cx="58" cy="18" r="3" fill="var(--art-accent)" fill-opacity="0.9" />
        <circle cx="94" cy="38" r="2.5" fill="var(--art-accent)" fill-opacity="0.6" />
        <circle cx="52" cy="50" r="2" fill="currentColor" fill-opacity="0.35" />
        <line
          x1="24"
          y1="36"
          x2="58"
          y2="18"
          stroke="currentColor"
          stroke-opacity="0.28"
          stroke-dasharray="2 2"
        />
        <line x1="58" y1="18" x2="94" y2="38" stroke="currentColor" stroke-opacity="0.32" />
        <line x1="24" y1="36" x2="52" y2="50" stroke="currentColor" stroke-opacity="0.22" />
        <line x1="52" y1="50" x2="94" y2="38" stroke="currentColor" stroke-opacity="0.18" />
      </template>

      <!-- 精密雷达同心刻度 -->
      <template v-else-if="theme.type === 'radar' || theme.type === 'rings'">
        <circle
          cx="96"
          cy="58"
          r="22"
          stroke="currentColor"
          stroke-opacity="0.26"
          stroke-width="1"
        />
        <circle
          cx="96"
          cy="58"
          r="42"
          stroke="currentColor"
          stroke-opacity="0.18"
          stroke-dasharray="3 3"
        />
        <circle cx="96" cy="58" r="62" stroke="currentColor" stroke-opacity="0.1" />
        <line
          x1="96"
          y1="58"
          x2="52"
          y2="18"
          stroke="var(--art-accent)"
          stroke-opacity="0.55"
          stroke-width="1.2"
        />
        <circle cx="52" cy="18" r="2.5" fill="var(--art-accent)" />
      </template>

      <!-- 工业坐标矩阵 -->
      <template v-else-if="theme.type === 'grid'">
        <path
          d="M16 18H104M16 34H104M16 50H104"
          stroke="currentColor"
          stroke-opacity="0.14"
          stroke-width="1"
        />
        <path
          d="M34 8V60M60 8V60M86 8V60"
          stroke="currentColor"
          stroke-opacity="0.14"
          stroke-width="1"
        />
        <rect
          x="55"
          y="29"
          width="10"
          height="10"
          rx="2"
          stroke="var(--art-accent)"
          stroke-opacity="0.75"
          stroke-width="1.2"
        />
        <circle cx="60" cy="34" r="2" fill="var(--art-accent)" />
      </template>

      <!-- 信号频谱波形 -->
      <template v-else>
        <path
          d="M12 48C32 36 48 54 72 38C86 28 96 38 108 30"
          stroke="var(--art-accent)"
          stroke-opacity="0.55"
          stroke-width="1.3"
        />
        <path
          d="M12 34C30 22 52 38 76 22C88 14 98 24 108 18"
          stroke="currentColor"
          stroke-opacity="0.22"
          stroke-width="1.1"
          stroke-dasharray="2 2"
        />
        <path
          d="M12 20C36 10 56 24 80 12C92 6 100 14 108 10"
          stroke="currentColor"
          stroke-opacity="0.12"
          stroke-width="1"
        />
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
  right: 8px;
  bottom: 0;
  width: 148px;
  height: 82px;
  color: var(--ink);
  transition: transform 0.35s cubic-bezier(0.22, 1, 0.36, 1);
}

:global(.card:hover) .art-svg {
  transform: translateY(-3px) scale(1.03);
}

.art-index-pill {
  position: relative;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 6px;
  border: 1px solid var(--line-strong, rgba(255, 255, 255, 0.14));
  background: color-mix(in srgb, var(--paper) 82%, transparent);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  box-shadow: var(--surface-highlight);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.5px;
  color: var(--muted);
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;
}

.pill-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--art-accent, var(--brand));
  opacity: 0.85;
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
