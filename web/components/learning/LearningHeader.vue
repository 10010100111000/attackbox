<script setup lang="ts">
const route = useRoute()
const isLearnActive = computed(() => route.path.startsWith('/learn'))
const isChallengesActive = computed(() => route.path.startsWith('/challenges'))
const isReferencesActive = computed(() => route.path.startsWith('/references'))
const isDiscussActive = computed(() => route.path.startsWith('/discuss'))
</script>

<template>
  <header class="learning-header">
    <div class="learning-frame header-inner">
      <div class="header-left">
        <NuxtLink to="/learn" class="brand">
          <img
            src="~/assets/images/logo.webp"
            alt="AttackBox Logo"
            class="brand-logo"
            width="31"
            height="31"
          />
          <span>AttackBox</span>
        </NuxtLink>
        <nav class="main-nav" aria-label="主导航">
          <NuxtLink to="/learn" :class="{ active: isLearnActive }">学习</NuxtLink>
          <NuxtLink to="/challenges" :class="{ active: isChallengesActive }">挑战</NuxtLink>
          <NuxtLink to="/references" :class="{ active: isReferencesActive }">参考资料</NuxtLink>
          <NuxtLink to="/discuss" :class="{ active: isDiscussActive }">讨论</NuxtLink>
        </nav>
      </div>

      <UserMenu />
    </div>
  </header>
</template>

<style scoped>
.learning-header {
  height: 68px;
  border-bottom: 1px solid var(--line);
  background: color-mix(in srgb, var(--paper) 88%, transparent);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  position: sticky;
  top: 0;
  z-index: 50;
  color: var(--ink);
}

.header-inner {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 36px;
  height: 100%;
}

.brand {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 18px;
  letter-spacing: -0.4px;
  color: var(--ink);
  text-decoration: none;
}

.brand-logo {
  display: block;
  transition: transform 0.18s ease;
  border-radius: 6px;
}

.main-nav {
  display: flex;
  align-items: center;
  gap: 28px;
  height: 100%;
}

.main-nav a {
  height: 100%;
  display: flex;
  align-items: center;
  border-bottom: 2px solid transparent;
  padding: 2px 2px 0;
  font-size: 14px;
  color: var(--muted);
  text-decoration: none;
  transition:
    color 0.15s ease,
    border-color 0.15s ease;
}

.main-nav a:hover {
  color: var(--ink);
}

.main-nav a.active {
  color: var(--ink);
  border-bottom-color: var(--learning-spark);
  font-weight: 600;
}

.brand:hover .brand-logo {
  transform: rotate(-6deg) translateY(-1px);
}
@media (prefers-reduced-motion: reduce) {
  .brand-logo {
    transition: none;
    transform: none !important;
  }
}
@media (max-width: 640px) {
  .header-left {
    gap: 14px;
    min-width: 0;
  }
  .brand > span {
    display: none;
  }
  .main-nav {
    gap: 12px;
  }
  .main-nav a {
    font-size: 13px;
    white-space: nowrap;
  }
}
</style>
