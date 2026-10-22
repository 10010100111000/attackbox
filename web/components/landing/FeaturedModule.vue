<script setup lang="ts">
import { ArrowUpRight } from 'lucide-vue-next'
import { handleCardPointerMove } from '~/composables/useModuleRail'
</script>

<template>
  <NuxtLink
    to="/auth?mode=login"
    class="module-card featured-module"
    data-module="cloud"
    data-level="advanced"
    style="--tone: #d8a57b; --glow: #d8a57b"
    aria-label="进入云原生攻防模块"
    @pointermove="handleCardPointerMove"
  >
    <div class="module-art cloud-art">
      <div class="card-topline">
        <span>03 / CLOUD NATIVE</span>
        <span class="level advanced">进阶</span>
      </div>
      <svg
        class="diagram"
        viewBox="0 0 400 176"
        fill="none"
        aria-hidden="true"
        focusable="false"
      >
        <path d="M80 66L200 30L321 66L200 104Z M80 66V105L200 145L321 105V66 M200 104V145 M140 48L260 86 M140 85L260 48" stroke="currentColor" stroke-width=".8"/>
        <path d="M140 48V87L200 107L260 87V48" stroke="currentColor" stroke-dasharray="3 5" opacity=".45"/>
        <circle cx="200" cy="30" r="4" fill="currentColor"/>
        <circle cx="80" cy="66" r="3" fill="currentColor"/>
        <circle cx="321" cy="66" r="3" fill="currentColor"/>
        <text x="170" y="88" fill="currentColor" stroke="none" font-size="10" font-family="monospace">CLUSTER</text>
      </svg>
    </div>
    <div class="card-copy">
      <h3>云原生攻防</h3>
      <p>沿着身份、服务与集群寻找攻击路径。</p>
      <div class="card-foot">
        <span>10 章节 · 5 次实践</span>
        <ArrowUpRight class="card-arrow" />
      </div>
    </div>
  </NuxtLink>
</template>
