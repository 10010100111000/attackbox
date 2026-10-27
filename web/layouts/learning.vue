<script setup lang="ts">
import LearningHeader from '~/components/learning/LearningHeader.vue'
</script>

<template>
  <div class="learning-layout">
    <div class="learning-backdrop" aria-hidden="true">
      <div class="aurora-orb orb-coral" />
      <div class="aurora-orb orb-rose" />
      <div class="aurora-dots" />
    </div>
    <LearningHeader />
    <main class="learning-main">
      <slot />
    </main>
  </div>
</template>

<style src="~/assets/css/learning-shell.css"></style>

<style scoped>
.learning-layout {
  min-height: 100dvh;
  background-color: var(--bg);
  color: var(--ink);
  display: flex;
  flex-direction: column;
  position: relative;
  isolation: isolate;
}

/* 顶部 2px 落日流光能量线 */
.learning-layout::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  z-index: 60;
  pointer-events: none;
  background: var(--gradient-collision);
  background-size: 200% 100%;
  animation: gradient-flow 7s ease infinite;
}

.learning-backdrop {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

/* 双色碰撞极光光团：左暖珊瑚橙，右电光玫红与星云紫 */
.aurora-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(72px);
  opacity: 0.85;
  animation: aurora-float 14s ease-in-out infinite alternate;
}

.orb-coral {
  top: -140px;
  left: 8%;
  width: 560px;
  height: 340px;
  background: radial-gradient(circle, rgba(255, 175, 136, 0.22) 0%, rgba(255, 140, 90, 0.08) 55%, transparent 75%);
}

.orb-rose {
  top: -110px;
  right: 6%;
  width: 520px;
  height: 340px;
  background: radial-gradient(circle, rgba(239, 124, 141, 0.18) 0%, rgba(192, 132, 252, 0.12) 55%, transparent 75%);
  animation-delay: -6s;
}

/* 富有设计感的暖金微点阵 + 十字坐标网格 */
.aurora-dots {
  position: absolute;
  inset: 0;
  background-image:
    radial-gradient(var(--bg-dot-color) 1px, transparent 1px),
    linear-gradient(to right, var(--bg-grid-line) 1px, transparent 1px),
    linear-gradient(to bottom, var(--bg-grid-line) 1px, transparent 1px);
  background-size: 24px 24px, 96px 96px, 96px 96px;
  mask-image: radial-gradient(ellipse 90% 68% at 50% 0%, #000 25%, transparent 88%);
  -webkit-mask-image: radial-gradient(ellipse 90% 68% at 50% 0%, #000 25%, transparent 88%);
}

.learning-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}
</style>
