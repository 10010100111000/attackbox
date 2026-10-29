<script setup lang="ts">
withDefaults(defineProps<{ inline?: boolean; label?: string }>(), {
  inline: false,
  label: '请稍候',
})
</script>

<template>
  <span class="busy-indicator" :class="{ inline }" role="status" :aria-label="label" aria-live="polite">
    <svg class="busy-orbit" viewBox="0 0 32 32" aria-hidden="true">
      <circle cx="16" cy="16" r="10" class="orbit-track" />
      <g class="orbit-dots">
        <circle cx="16" cy="6" r="3.5" />
        <circle cx="16" cy="26" r="2.5" />
      </g>
    </svg>
  </span>
</template>

<style scoped>
.busy-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 160px;
  color: var(--green, var(--brand, currentColor));
  animation: busy-appear 0.12s ease 0.18s both;
}

.busy-indicator.inline {
  display: inline-flex;
  min-height: 0;
  width: 1.2em;
  height: 1.2em;
  vertical-align: middle;
  flex-shrink: 0;
  color: currentColor;
}

.busy-orbit {
  width: 32px;
  height: 32px;
  overflow: visible;
}

.inline .busy-orbit {
  width: 100%;
  height: 100%;
}

.orbit-track {
  fill: none;
  stroke: currentColor;
  stroke-width: 1;
  opacity: 0.18;
}

.orbit-dots {
  fill: currentColor;
  transform-origin: 16px 16px;
  animation: busy-turn 1.3s cubic-bezier(0.45, 0.1, 0.55, 0.9) infinite;
}

.orbit-dots circle:last-child {
  opacity: 0.45;
}

@keyframes busy-turn {
  to {
    transform: rotate(360deg);
  }
}

@keyframes busy-appear {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .orbit-dots {
    animation: none;
  }
  .busy-indicator {
    animation: busy-appear 0s linear 0.18s both;
  }
}
</style>
