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

<style scoped>
.hero {
  height: 680px;
  position: relative;
  overflow: hidden;
  isolation: isolate;
}

.hero > .wrap {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  position: relative;
  z-index: 3;
  pointer-events: none;
}

.hero-copy {
  pointer-events: auto;
}

.hero h1 {
  font-size: 84px;
  line-height: 1.16;
  letter-spacing: -4px;
  font-weight: 650;
  margin-bottom: 35px;
}

.hero h1 span {
  display: block;
}

.hero h1 .warm {
  background: var(--gradient-warm, linear-gradient(100deg, #ffe0b0, #ffae82 48%, #ef7c8d));
  background-clip: text;
  -webkit-background-clip: text;
  color: transparent;
  -webkit-text-fill-color: transparent;
}

.hero-actions {
  display: flex;
  justify-content: center;
}

.primary {
  background: var(--btn-primary-bg, #f4f4f6);
  color: var(--btn-primary-fg, #090a0e);
  box-shadow: var(--btn-primary-shadow);
  padding: 13px 20px;
  border-radius: var(--radius-md, 7px);
  font-size: 12px;
  font-weight: 600;
  display: inline-flex;
  gap: 38px;
  align-items: center;
  transition:
    transform 0.2s cubic-bezier(0.22, 1, 0.36, 1),
    background 0.2s,
    box-shadow 0.2s;
}

.primary:hover {
  transform: translateY(-2px);
  background: var(--btn-primary-hover, #ffffff);
}

.hero-vignette {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  background:
    radial-gradient(ellipse at 50% 47%, #090a0ef2 0%, #090a0ee3 26%, #090a0e77 58%, #090a0e44 80%),
    linear-gradient(0deg, #090a0e, transparent 22%, transparent 80%, #090a0e);
}

.hero-grain {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  opacity: 0.1;
  background: repeating-linear-gradient(0deg, transparent 0 3px, rgba(0, 0, 0, 0.27) 3px 4px);
  mask-image: linear-gradient(0deg, transparent, #000, transparent);
  -webkit-mask-image: linear-gradient(0deg, transparent, #000, transparent);
}

@media (max-width: 1100px) {
  .hero {
    height: 630px;
  }
  .hero h1 {
    font-size: 76px;
  }
}

@media (max-width: 760px) {
  .hero {
    height: 640px;
  }
  .hero h1 {
    font-size: 57px;
    letter-spacing: -3px;
    margin-bottom: 29px;
  }
  .primary {
    font-size: 11px;
    gap: 30px;
    padding: 12px 16px;
  }
}

@media (max-width: 380px) {
  .hero h1 {
    font-size: 51px;
  }
  .hero {
    height: 610px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .primary {
    transition: none;
  }
}
</style>
