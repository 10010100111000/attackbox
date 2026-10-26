<script setup lang="ts">
const route = useRoute()
const isLearnActive = computed(() => route.path.startsWith('/learn'))
const isChallengesActive = computed(() => route.path.startsWith('/challenges'))
const isDiscussActive = computed(() => route.path.startsWith('/discuss'))
</script>

<template>
  <header class="learning-header">
    <div class="header-left">
      <NuxtLink to="/learn" class="brand">
        <img src="/logo.png" alt="AttackBox Logo" class="brand-logo" width="31" height="31" />
        <span>AttackBox</span>
      </NuxtLink>
      <nav class="main-nav" aria-label="主导航">
        <NuxtLink to="/learn" :class="{ active: isLearnActive }">学习</NuxtLink>
        <NuxtLink to="/challenges" :class="{ active: isChallengesActive }">挑战</NuxtLink>
        <NuxtLink to="/discuss" :class="{ active: isDiscussActive }">讨论</NuxtLink>
      </nav>
    </div>

    <UserMenu />
  </header>
</template>

<style scoped>
.learning-header {
  height: 68px;
  border-bottom: 1px solid var(--line);
  background: var(--paper);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4%;
  position: sticky;
  top: 0;
  z-index: 50;
  color: var(--ink);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 36px;
  height: 100%;
}

.brand {
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
  transition: color 0.15s ease, border-color 0.15s ease;
}

.main-nav a:hover {
  color: var(--ink);
}

.main-nav a.active {
  color: var(--ink);
  border-bottom-color: var(--ink);
  font-weight: 600;
}

@media (max-width: 640px) {
  .learning-header {
    padding: 0 16px;
  }
  .header-left { gap: 14px; min-width: 0; }
  .brand > span { display: none; }
  .brand { flex-shrink: 0; }
  .main-nav { gap: 12px; }
  .main-nav a { font-size: 13px; white-space: nowrap; }
}
</style>
