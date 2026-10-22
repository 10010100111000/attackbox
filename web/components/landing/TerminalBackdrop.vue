<script setup lang="ts">
import { useTerminalPlayer } from '~/composables/useTerminalPlayer'

const {
  branch,
  typedCommand,
  showCursor,
  feedLines,
  isClearing
} = useTerminalPlayer()
</script>

<template>
  <div class="terminal-backdrop" aria-hidden="true">
    <div class="terminal-screen">
      <div class="terminal-feed" :class="{ 'is-clearing': isClearing }">
        <div class="terminal-line dim quiet">// {{ branch }}</div>
        <div class="terminal-line command">$ {{ typedCommand }}<span v-if="showCursor" class="cursor" /></div>
        <div
          v-for="(row, idx) in feedLines"
          :key="idx"
          class="terminal-line"
          :class="row.type"
        >
          {{ row.text }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.terminal-backdrop {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  opacity: 0.62;
  filter: blur(1.4px);
  background: radial-gradient(ellipse at 18% 50%, #a0674917, transparent 55%);
}

.terminal-screen {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  padding: 68px max(55px, calc((100vw - 1240px) / 2));
  overflow: hidden;
  font: 18px / 34px "SFMono-Regular", Consolas, monospace;
  white-space: pre;
  color: #b8a9b8;
}

.terminal-feed {
  transition: opacity 0.22s;
}

.terminal-feed.is-clearing {
  opacity: 0;
}

.terminal-line {
  height: 34px;
  white-space: pre;
}

.terminal-line.command {
  font-size: 19px;
  color: #ddbc99;
}

.terminal-line.dim {
  color: #938797;
}

.terminal-line.quiet {
  opacity: 0.6;
}

.terminal-line.accent {
  color: #c99e80;
}

.terminal-line.success {
  color: #aac1ae;
}

.cursor {
  display: inline-block;
  width: 9px;
  height: 19px;
  background: #f1b998;
  vertical-align: -2px;
  box-shadow: 0 0 12px #e3b28566;
  animation: blink 0.8s steps(1) infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

@media (max-width: 760px) {
  .terminal-backdrop {
    opacity: 0.57;
    filter: blur(1.2px);
  }

  .terminal-screen {
    padding: 60px 17px;
    font-size: 12px;
    line-height: 30px;
  }

  .terminal-line {
    height: 30px;
  }

  .terminal-line.command {
    font-size: 12px;
  }

  .cursor {
    height: 13px;
    width: 7px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .terminal-feed {
    transition: none;
  }
  .cursor {
    animation: none;
  }
}
</style>
