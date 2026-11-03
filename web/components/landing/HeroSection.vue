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
  line-height: 1.14;
  letter-spacing: -4px;
  font-weight: 700;
  color: var(--ink);
  margin: 0 0 36px;
}

.hero h1 span {
  display: block;
}

.hero h1 .warm {
  background: var(
    --gradient-warm,
    linear-gradient(100deg, #ffedd5 0%, #fdba74 36%, #fb923c 68%, #f97316 100%)
  );
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
  background: #fafafa;
  color: #09090b;
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.6),
    0 10px 28px -4px rgba(249, 115, 22, 0.22),
    inset 0 1px 0 #ffffff;
  padding: 13px 22px;
  border-radius: var(--radius-md, 7px);
  font-size: 13px;
  font-weight: 700;
  text-decoration: none;
  display: inline-flex;
  gap: 36px;
  align-items: center;
  transition:
    transform 0.2s cubic-bezier(0.22, 1, 0.36, 1),
    background 0.2s,
    box-shadow 0.2s;
}

.primary:hover {
  transform: translateY(-2px);
  background: #ffffff;
  box-shadow:
    0 4px 12px rgba(0, 0, 0, 0.65),
    0 14px 36px -4px rgba(249, 115, 22, 0.36),
    inset 0 1px 0 #ffffff;
}

.hero-vignette {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  background:
    radial-gradient(
      ellipse 95% 85% at 50% 48%,
      rgba(9, 9, 11, 0.24) 0%,
      rgba(9, 9, 11, 0.34) 36%,
      rgba(9, 9, 11, 0.52) 66%,
      rgba(9, 9, 11, 0.78) 88%,
      #09090b 100%
    ),
    linear-gradient(
      180deg,
      #09090b 0%,
      rgba(9, 9, 11, 0.62) 12%,
      rgba(9, 9, 11, 0.16) 28%,
      rgba(9, 9, 11, 0.08) 50%,
      rgba(9, 9, 11, 0.2) 72%,
      rgba(9, 9, 11, 0.68) 88%,
      #09090b 100%
    );
}

.hero-grain {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  opacity: 0.06;
  background: repeating-linear-gradient(0deg, transparent 0 3px, rgba(0, 0, 0, 0.24) 3px 4px);
  mask-image: linear-gradient(0deg, transparent, #000 25%, #000 75%, transparent);
  -webkit-mask-image: linear-gradient(0deg, transparent, #000 25%, #000 75%, transparent);
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
    font-size: 56px;
    letter-spacing: -3px;
    margin-bottom: 28px;
  }
  .primary {
    font-size: 12px;
    gap: 28px;
    padding: 12px 18px;
  }
}

@media (max-width: 380px) {
  .hero h1 {
    font-size: 50px;
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
