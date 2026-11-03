<template>
  <div class="ambient-backdrop" aria-hidden="true">
    <div class="backdrop-spotlight" />
    <div class="backdrop-grain" />
  </div>
</template>

<style scoped>
.ambient-backdrop {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

.backdrop-spotlight {
  position: absolute;
  inset: 0;
  background-image: var(--backdrop-glow), var(--backdrop-secondary-glow);
}

.backdrop-grain {
  position: absolute;
  inset: 0;
  opacity: 0.024;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.82' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  background-repeat: repeat;
  background-size: 180px 180px;
}
</style>
