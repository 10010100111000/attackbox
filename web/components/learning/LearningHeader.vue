<script setup lang="ts">
const route = useRoute()
const isLearnActive = computed(() => route.path.startsWith('/learn'))
const isChallengesActive = computed(() => route.path.startsWith('/challenges'))
const isReferencesActive = computed(() => route.path.startsWith('/references'))
const isDiscussActive = computed(() => route.path.startsWith('/discuss'))
</script>

<template>
  <SiteHeader brand-to="/learn" class="learning-header">
    <nav class="main-nav" aria-label="主导航">
      <NuxtLink to="/learn" :class="{ active: isLearnActive }">学习</NuxtLink>
      <NuxtLink to="/challenges" :class="{ active: isChallengesActive }">挑战</NuxtLink>
      <NuxtLink to="/references" :class="{ active: isReferencesActive }">参考资料</NuxtLink>
      <NuxtLink to="/discuss" :class="{ active: isDiscussActive }">讨论</NuxtLink>
    </nav>
  </SiteHeader>
</template>

<style scoped>
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

@media (max-width: 640px) {
  .main-nav {
    gap: 12px;
  }
  .main-nav a {
    font-size: 13px;
    white-space: nowrap;
  }
}
</style>
