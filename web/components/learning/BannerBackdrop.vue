<script setup lang="ts">
/** Stable across SSR, hydration and editor preview; no remote assets or animation. */
const props = defineProps<{ seed: string }>()
const identity = computed(() => {
  let hash = 2166136261
  for (const char of props.seed) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619) >>> 0
  return { color: ['#198c87', '#587bd1', '#b56745', '#8a62b4'][hash % 4], motif: (hash >>> 8) % 3 }
})
</script>

<template>
  <div class="banner-backdrop" :style="{ '--banner-accent': identity.color }" aria-hidden="true">
    <svg viewBox="0 0 1200 240" preserveAspectRatio="xMidYMid slice" fill="none">
      <g v-if="identity.motif === 0" class="contours">
        <ellipse
          v-for="n in 7"
          :key="n"
          cx="1000"
          cy="130"
          :rx="70 + n * 40"
          :ry="25 + n * 18"
          transform="rotate(-24 1000 130)"
        />
      </g>
      <g v-else-if="identity.motif === 1" class="circuits">
        <path
          d="M720 0v60h130l60 60h290M780 240v-60h120l60-60M880 0v40h110l70 70h140M960 240v-40h110l60-60h70"
        />
        <circle cx="850" cy="60" r="5" />
        <circle cx="900" cy="180" r="5" />
        <circle cx="990" cy="40" r="5" />
        <circle cx="1070" cy="200" r="5" />
      </g>
      <g v-else class="orbits" transform="translate(990 120)">
        <circle v-for="n in 5" :key="n" :r="n * 42" />
        <path d="M-280 0h560M0-200v400" />
        <circle cx="126" cy="0" r="7" class="orbit-point" />
        <circle cx="-84" cy="0" r="5" class="orbit-point" />
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
      ellipse at 86% 35%,
      color-mix(in srgb, var(--banner-accent) 24%, transparent),
      transparent 65%
    ),
    linear-gradient(115deg, var(--paper), color-mix(in srgb, var(--banner-accent) 9%, var(--paper)));
}
svg {
  width: 100%;
  height: 100%;
  position: absolute;
  inset: 0;
  stroke: currentColor;
  stroke-width: 1;
  opacity: 0.28;
}
.contours {
  stroke-width: 1.5;
}
.circuits {
  stroke-width: 1.5;
}
.orbit-point {
  fill: currentColor;
}
@media (max-width: 640px) {
  svg {
    opacity: 0.14;
  }
}
</style>
