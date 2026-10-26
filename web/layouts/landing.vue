<script setup lang="ts">
const {currentUser}=useAuth()
</script>
<template>
  <div class="landing-layout" data-theme="midnight">
    <div class="wrap">
      <header class="header">
        <NuxtLink to="/" class="brand">
          <img src="/logo.png" alt="AttackBox Logo" class="brand-logo" width="31" height="31" />
          <span>AttackBox</span>
        </NuxtLink>
        <UserMenu v-if="currentUser"/><NuxtLink v-else to="/auth" class="platform-link">
          <span>进入平台</span>
        </NuxtLink>
      </header>
    </div>

    <main>
      <slot />
    </main>

    <div class="wrap">
      <footer class="footer">
        <NuxtLink to="/" class="footer-brand">
          AttackBox
        </NuxtLink>
        <span class="copyright">© 2026 AttackBox</span>
      </footer>
    </div>
  </div>
</template>

<style>
/* Landing Page Styles scoped to .landing-layout */
.landing-layout {
  min-height: 100vh;
  background: var(--bg-app);
  color: var(--text-main);
  overflow-x: clip;

  .wrap {
    width: min(1240px, calc(100% - 100px));
    margin: auto;
  }

  .header {
    height: 88px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--border-line, rgba(255, 255, 255, 0.07));
    position: relative;
    z-index: 5;
  }

  .brand {
    display: flex;
    gap: 10px;
    align-items: center;
    font-size: 19px;
    font-weight: 650;
    letter-spacing: -0.6px;
  }

  .brand-logo {
    width: 31px;
    height: 31px;
    border-radius: var(--radius-md, 8px);
    object-fit: contain;
    display: block;
  }


  .platform-link {
    border: 1px solid rgba(255, 255, 255, 0.15);
    padding: 8px 14px;
    border-radius: var(--radius-md, 7px);
    display: flex;
    gap: 22px;
    font-size: 11px;
    transition: background 0.2s;
  }

  .platform-link:hover {
    background: rgba(255, 255, 255, 0.05);
  }

  /* Hero Section */
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
    background: var(--accent-coral, #ffaf88);
    color: var(--text-on-accent, #271a15);
    padding: 13px 19px;
    border-radius: var(--radius-md, 7px);
    font-size: 12px;
    font-weight: 550;
    display: inline-flex;
    gap: 38px;
    align-items: center;
    transition: transform 0.2s, background 0.2s;
  }

  .primary:hover {
    transform: translateY(-2px);
    background: var(--accent-hover, #ffc19e);
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

  /* Modules Section */
  .modules {
    padding: 55px 0 74px;
    position: relative;
  }

  .modules:before {
    content: '';
    position: absolute;
    top: 0;
    left: 20%;
    right: 20%;
    height: 1px;
    background: linear-gradient(90deg, transparent, #c19e7f33, transparent);
  }

  .section-head {
    text-align: center;
    margin-bottom: 38px;
  }

  .section-head h2 {
    font-size: 34px;
    font-weight: 550;
    letter-spacing: -1.3px;
  }

  .module-card {
    --tone: #c1a381;
    --glow: #d3a275;
    position: relative;
    border: 1px solid var(--border-line, rgba(255, 255, 255, 0.07));
    background: var(--bg-card, #111217);
    border-radius: var(--radius-lg, 11px);
    overflow: hidden;
    isolation: isolate;
    display: block;
    transition: transform 0.3s, border-color 0.3s, box-shadow 0.3s;
  }

  .module-card:hover {
    transform: translateY(-4px);
    border-color: color-mix(in srgb, var(--tone) 50%, transparent);
    box-shadow: var(--card-glow, 0 15px 45px rgba(0, 0, 0, 0.4));
  }

  .module-art {
    height: 163px;
    position: relative;
    overflow: hidden;
    background: var(--bg-input, #121318);
  }

  .module-art:after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(0deg, var(--bg-card, #111217), transparent 75%);
  }

  .module-art:before {
    content: '';
    position: absolute;
    inset: 0;
    z-index: 2;
    background: radial-gradient(280px circle at var(--mx, 50%) var(--my, 40%), color-mix(in srgb, var(--glow) 14%, transparent), transparent 75%);
    opacity: 0;
    transition: opacity 0.35s;
  }

  .module-card:hover .module-art:before {
    opacity: 1;
  }

  .module-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    opacity: 0.45;
    filter: saturate(0.35);
    transition: transform 0.7s, opacity 0.5s;
  }

  .module-card:hover img {
    transform: scale(1.07);
    opacity: 0.65;
  }

  .art-code {
    position: absolute;
    top: 37px;
    left: 30px;
    right: 20px;
    z-index: 1;
    font: 12px / 1.9 monospace;
    color: var(--tone);
    transform: translateY(7px);
    transition: transform 0.5s;
    white-space: pre;
  }

  .module-card:hover .art-code {
    transform: translateY(0);
  }

  .art-code .quiet {
    opacity: 0.35;
  }

  .art-code .bright {
    color: #efe1d4;
  }

  .card-topline {
    position: absolute;
    top: 17px;
    left: 21px;
    right: 21px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font: 8px monospace;
    letter-spacing: 1.4px;
    color: #99909b;
    z-index: 3;
  }

  .level {
    font: 9px / 1.5 -apple-system, sans-serif;
    letter-spacing: 0;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: var(--radius-sm, 4px);
    padding: 2px 6px;
    color: #d2b29c;
    background: rgba(23, 21, 26, 0.5);
  }

  .level.advanced {
    color: var(--tone);
    border-color: color-mix(in srgb, var(--tone) 25%, transparent);
  }

  .card-copy {
    padding: 0 23px 21px;
    position: relative;
    z-index: 3;
  }

  .card-copy h3 {
    font-size: 20px;
    letter-spacing: -0.5px;
    font-weight: 550;
    margin-bottom: 9px;
  }

  .card-copy p {
    font-size: 11px;
    color: #8c8593;
    margin-bottom: 20px;
  }

  .card-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 9px;
    color: #7b7582;
    border-top: 1px solid rgba(255, 255, 255, 0.04);
    padding-top: 13px;
  }



  .cloud-art {
    background: radial-gradient(ellipse at 55% 30%, rgba(138, 91, 71, 0.2), transparent 66%), #121318;
  }

  .reverse-art {
    background: radial-gradient(ellipse at 45% 30%, rgba(103, 86, 134, 0.19), transparent 65%), #121318;
  }

  .container-art {
    background: radial-gradient(ellipse at 60% 20%, rgba(69, 119, 120, 0.19), transparent 65%), #121318;
  }

  .diagram {
    position: absolute;
    width: 100%;
    height: 100%;
    color: var(--tone);
    opacity: 0.65;
    transition: transform 0.7s;
  }

  .module-card:hover .diagram {
    transform: translateY(-5px);
  }

  .featured-module {
    display: grid;
    grid-template-columns: 1.25fr 1fr;
    min-height: 310px;
    margin-bottom: 23px;
    background: linear-gradient(120deg, #18171c, var(--bg-card, #111217));
    border-color: rgba(195, 154, 123, 0.13);
  }

  .featured-module .module-art {
    height: 100%;
    min-height: 310px;
    background: radial-gradient(ellipse at 50% 50%, rgba(163, 107, 66, 0.19), transparent 68%);
  }

  .featured-module .module-art:after {
    background: linear-gradient(90deg, transparent 65%, #141419);
  }

  .featured-module .card-copy {
    padding: 46px;
    align-self: center;
  }

  .featured-module .card-copy h3 {
    font-size: 32px;
    letter-spacing: -1px;
    margin-bottom: 15px;
  }

  .featured-module .card-copy p {
    font-size: 12px;
    line-height: 1.9;
    max-width: 250px;
    margin-bottom: 31px;
  }

  .featured-module .card-foot {
    font-size: 10px;
  }

  .featured-module .diagram {
    transform: scale(0.93);
    opacity: 0.9;
  }

  .featured-module:hover .diagram {
    transform: scale(0.98) translateY(-3px);
  }

  .module-rail {
    display: flex;
    gap: 20px;
    overflow-x: auto;
    overflow-y: hidden;
    padding: 6px 1px 15px;
    scrollbar-width: none;
    overscroll-behavior-x: contain;
    overflow-anchor: none;
  }

  .module-rail::-webkit-scrollbar {
    display: none;
  }

  .module-rail > .module-card {
    flex: 0 0 31%;
  }

  .module-rail:focus-visible {
    outline: 1px solid rgba(195, 154, 123, 0.4);
    outline-offset: 6px;
  }

  .footer {
    border-top: 1px solid var(--border-line, rgba(255, 255, 255, 0.07));
    padding: 26px 0 30px;
    display: flex;
    justify-content: space-between;
    color: #716777;
    font-size: 10px;
  }

  .footer-brand {
    color: #a99cae;
    font-size: 12px;
  }

  @media (max-width: 1100px) {
    .wrap {
      width: calc(100% - 60px);
    }
    .hero {
      height: 630px;
    }
    .hero h1 {
      font-size: 76px;
    }
    .featured-module .card-copy {
      padding: 32px;
    }
    .featured-module .card-copy h3 {
      font-size: 28px;
    }
    .module-rail > .module-card {
      flex-basis: 38%;
    }
    .card-copy {
      padding: 0 18px 18px;
    }
    .card-copy h3 {
      font-size: 19px;
    }
    .card-copy p {
      font-size: 10px;
    }
    .art-code {
      font-size: 10px;
      left: 23px;
    }
  }

  @media (max-width: 760px) {
    .wrap {
      width: calc(100% - 40px);
    }
    .header {
      height: 73px;
    }
    .brand {
      font-size: 16px;
      gap: 7px;
    }
    .brand-logo {
      height: 27px;
      width: 27px;
      font-size: 19px;
      border-radius: var(--radius-md, 7px);
    }
    .platform-link {
      font-size: 10px;
      padding: 7px 10px;
      gap: 10px;
    }
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
    .modules {
      padding: 35px 0 42px;
    }
    .section-head {
      margin-bottom: 27px;
    }
    .section-head h2 {
      font-size: 27px;
    }
    .featured-module {
      grid-template-columns: 1fr;
      min-height: 0;
      margin-bottom: 16px;
    }
    .featured-module .module-art {
      height: 200px;
      min-height: 0;
    }
    .featured-module .module-art:after {
      background: linear-gradient(0deg, #141419, transparent 70%);
    }
    .featured-module .card-copy {
      padding: 0 25px 24px;
    }
    .featured-module .card-copy h3 {
      font-size: 27px;
      margin-bottom: 11px;
    }
    .featured-module .card-copy p {
      max-width: 100%;
      font-size: 11px;
      margin-bottom: 20px;
    }
    .module-rail {
      gap: 14px;
      margin-right: -20px;
      padding-right: 20px;
      padding-bottom: 17px;
    }
    .module-rail > .module-card {
      flex-basis: 82%;
    }
    .module-rail .module-art {
      height: 157px;
    }
    .module-rail .card-copy {
      padding: 0 21px 20px;
    }
    .module-rail .card-copy h3 {
      font-size: 20px;
    }
    .module-rail .art-code {
      font-size: 11px;
    }
    .footer {
      font-size: 10px;
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
    .primary,
    .module-card,
    .module-art img,
    .module-art:before,
    .art-code,
    .diagram {
      transition: none;
    }
  }
}
</style>
