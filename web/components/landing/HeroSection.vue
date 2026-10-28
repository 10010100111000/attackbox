<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'
import TerminalBackdrop from './TerminalBackdrop.vue'
</script>

<template>
  <section class="hero">
    <TerminalBackdrop />
    <div class="hero-vignette" />
    <div class="hero-grain" aria-hidden="true" />
    <div class="wrap">
      <div class="hero-copy">
        <h1>
          <span>理解原理。</span>
          <span class="warm">亲手验证。</span>
        </h1>
        <div class="hero-actions">
          <NuxtLink to="/learn" class="primary">
            <span>开始学习</span>
            <ArrowRight class="w-4 h-4" />
          </NuxtLink>
        </div>
      </div>
    </div>
  </section>
</template>
