<script setup lang="ts">
import LearningHeader from '~/components/learning/LearningHeader.vue'
</script>

<template>
  <div class="learning-layout">
    <!-- 动态精密工程坐标网格 + 拓扑流光脉冲背景 -->
    <div class="learning-backdrop" aria-hidden="true">
      <div class="backdrop-spotlight" />
      <svg
        class="backdrop-circuit"
        viewBox="0 0 1440 900"
        preserveAspectRatio="xMidYMin slice"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <!-- 30px 细分辅助格 + 120px 主坐标格 + 十字准星锚点 -->
          <pattern id="ab-minor-grid" width="30" height="30" patternUnits="userSpaceOnUse">
            <path d="M 30 0 L 0 0 0 30" stroke="var(--grid-minor)" stroke-width="1" />
          </pattern>
          <pattern id="ab-major-grid" width="120" height="120" patternUnits="userSpaceOnUse">
            <rect width="120" height="120" fill="url(#ab-minor-grid)" />
            <path d="M 120 0 L 0 0 0 120" stroke="var(--grid-major)" stroke-width="1" />
            <!-- 坐标交点十字准星 (+) -->
            <path d="M -4 0 L 4 0 M 0 -4 L 0 4" stroke="var(--grid-cross)" stroke-width="1.2" />
          </pattern>

          <!-- 渐隐遮罩：让网格与流光在页面上半部清晰呈现，向四周和底部自然消隐 -->
          <radialGradient id="ab-fade-mask" cx="50%" cy="18%" r="72%">
            <stop offset="0%" stop-color="#ffffff" stop-opacity="1" />
            <stop offset="55%" stop-color="#ffffff" stop-opacity="0.65" />
            <stop offset="100%" stop-color="#ffffff" stop-opacity="0" />
          </radialGradient>
          <mask id="ab-circuit-mask">
            <rect width="1440" height="900" fill="url(#ab-fade-mask)" />
          </mask>

          <!-- 流光脉冲渐变尾迹 (暖铜金 / 雾钢蓝 / 亮银白) -->
          <linearGradient id="ab-beam-warm" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="var(--beam-warm)" stop-opacity="0" />
            <stop offset="50%" stop-color="var(--beam-warm)" stop-opacity="0.85" />
            <stop offset="100%" stop-color="#ffffff" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="ab-beam-cool" x1="100%" y1="0%" x2="0%" y2="0%">
            <stop offset="0%" stop-color="var(--beam-cool)" stop-opacity="0" />
            <stop offset="50%" stop-color="var(--beam-cool)" stop-opacity="0.8" />
            <stop offset="100%" stop-color="#ffffff" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="ab-beam-vert" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="var(--beam-silver)" stop-opacity="0" />
            <stop offset="55%" stop-color="var(--beam-warm)" stop-opacity="0.75" />
            <stop offset="100%" stop-color="var(--beam-silver)" stop-opacity="0" />
          </linearGradient>
        </defs>

        <g mask="url(#ab-circuit-mask)">
          <!-- 底层精密坐标网格 -->
          <rect width="1440" height="900" fill="url(#ab-major-grid)" />

          <!-- 静态拓扑折线轨道 (微弱底纹) -->
          <g stroke="var(--grid-major)" stroke-width="1.2">
            <path d="M 0 120 H 360 L 480 240 H 960 L 1080 120 H 1440" />
            <path d="M 0 360 H 240 L 360 240 H 720 L 840 360 H 1440" />
            <path d="M 240 0 V 240 L 360 360 V 720" />
            <path d="M 1200 0 V 240 L 1080 360 V 720" />
          </g>

          <!-- 动态游走的光脉冲流线 (沿着拓扑折线与网格平滑游走) -->
          <path
            class="circuit-beam beam-1"
            d="M 0 120 H 360 L 480 240 H 960 L 1080 120 H 1440"
            stroke="url(#ab-beam-warm)"
            stroke-width="1.6"
          />
          <path
            class="circuit-beam beam-2"
            d="M 1440 360 H 840 L 720 240 H 360 L 240 360 H 0"
            stroke="url(#ab-beam-cool)"
            stroke-width="1.5"
          />
          <path
            class="circuit-beam beam-3"
            d="M 0 480 H 600 L 720 360 H 1200 L 1320 480 H 1440"
            stroke="url(#ab-beam-warm)"
            stroke-width="1.4"
          />
          <path
            class="circuit-beam beam-4"
            d="M 240 0 V 240 L 360 360 V 720"
            stroke="url(#ab-beam-vert)"
            stroke-width="1.5"
          />
          <path
            class="circuit-beam beam-5"
            d="M 1200 0 V 240 L 1080 360 V 720"
            stroke="url(#ab-beam-vert)"
            stroke-width="1.5"
          />

          <!-- 拓扑转折点呼吸信标节点 -->
          <circle class="circuit-node node-1" cx="360" cy="120" r="2.5" fill="var(--beam-warm)" />
          <circle class="circuit-node node-2" cx="480" cy="240" r="2.5" fill="var(--beam-warm)" />
          <circle class="circuit-node node-3" cx="960" cy="240" r="2.5" fill="var(--beam-cool)" />
          <circle class="circuit-node node-4" cx="720" cy="240" r="2.5" fill="var(--beam-cool)" />
          <circle class="circuit-node node-5" cx="1080" cy="360" r="2.5" fill="var(--beam-warm)" />
        </g>
      </svg>
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

.learning-backdrop {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

.backdrop-spotlight {
  position: absolute;
  inset: 0;
  background: var(--backdrop-glow);
}

.backdrop-circuit {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.circuit-beam {
  stroke-dasharray: 220 1600;
  stroke-linecap: round;
  filter: drop-shadow(0 0 6px var(--beam-warm));
}

.beam-1 {
  animation: beam-flow 9s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

.beam-2 {
  filter: drop-shadow(0 0 6px var(--beam-cool));
  animation: beam-flow-reverse 11s cubic-bezier(0.4, 0, 0.2, 1) infinite;
  animation-delay: -3s;
}

.beam-3 {
  animation: beam-flow 13s cubic-bezier(0.4, 0, 0.2, 1) infinite;
  animation-delay: -6s;
}

.beam-4 {
  animation: beam-flow 10s cubic-bezier(0.4, 0, 0.2, 1) infinite;
  animation-delay: -2s;
}

.beam-5 {
  filter: drop-shadow(0 0 6px var(--beam-cool));
  animation: beam-flow 12s cubic-bezier(0.4, 0, 0.2, 1) infinite;
  animation-delay: -5s;
}

.circuit-node {
  transform-box: fill-box;
  transform-origin: center;
  animation: node-pulse 4s ease-in-out infinite;
}

.node-2 { animation-delay: -1.2s; }
.node-3 { animation-delay: -2.4s; }
.node-4 { animation-delay: -0.8s; }
.node-5 { animation-delay: -3.1s; }

.learning-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}
</style>
