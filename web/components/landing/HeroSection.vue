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
        <div class="hero-eyebrow" aria-hidden="true">
          <span class="eyebrow-dot" />
          <span>ATTACKBOX // SECURITY LAB</span>
        </div>
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
  display: flex;
  flex-direction: column;
  align-items: center;
}

.hero-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  margin-bottom: 24px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(18, 18, 21, 0.75);
  box-shadow: var(--surface-highlight), 0 8px 20px -4px rgba(0, 0, 0, 0.6);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1.2px;
  color: var(--body);
}

.eyebrow-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 0 10px var(--brand);
}

.hero h1 {
  font-size: 84px;
  line-height: 1.12;
  letter-spacing: -4px;
  font-weight: 700;
  color: var(--ink);
  margin: 0 0 36px;
}

.hero h1 span {
  display: block;
}

.hero h1 .warm {
  background: var(--gradient-warm, linear-gradient(100deg, #ffffff 0%, #fed7aa 48%, #f97316 100%));
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
  font-weight: 650;
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
      ellipse 70% 380px at 50% 0%,
      rgba(255, 255, 255, 0.06),
      rgba(249, 115, 22, 0.04) 48%,
      transparent 78%
    ),
    radial-gradient(ellipse at 50% 48%, #09090bf0 0%, #09090be0 28%, #09090b88 60%, #09090b55 82%),
    linear-gradient(0deg, #09090b, transparent 24%, transparent 78%, #09090b);
}

.hero-grain {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  opacity: 0.026;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.82' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  background-repeat: repeat;
  background-size: 180px 180px;
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
