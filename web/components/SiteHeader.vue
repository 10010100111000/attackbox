<script setup lang="ts">
withDefaults(defineProps<{ brandTo?: string }>(), { brandTo: '/' })
</script>

<template>
  <header class="site-header" :class="{ 'with-navigation': !!$slots.default }">
    <div class="header-inner">
      <div class="header-left">
        <NuxtLink :to="brandTo" class="brand">
          <img
            src="~/assets/images/logo.webp"
            alt="AttackBox Logo"
            class="brand-logo"
            width="28"
            height="28"
          />
          <span>AttackBox</span>
        </NuxtLink>
        <slot />
      </div>
      <div class="header-actions">
        <slot name="actions"><UserMenu /></slot>
      </div>
    </div>
  </header>
</template>

<style scoped>
.site-header {
  height: var(--site-header-height);
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: 50;
  border-bottom: 1px solid var(--line);
  background: color-mix(in srgb, var(--bg-app) 90%, transparent);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  color: var(--text-main);
}
.header-inner {
  width: 100%;
  height: 100%;
  padding-inline: var(--site-gutter);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 32px;
  min-width: 0;
  height: 100%;
}
.brand {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 9px;
  font-weight: 650;
  font-size: 17px;
  letter-spacing: -0.4px;
  color: inherit;
  text-decoration: none;
}
.brand-logo {
  width: 28px;
  height: 28px;
  display: block;
  object-fit: contain;
  border-radius: 6px;
  transition: transform 0.18s ease;
}
.brand:hover .brand-logo {
  transform: rotate(-6deg) translateY(-1px);
}
.header-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
@media (max-width: 640px) {
  .header-left {
    gap: 16px;
  }
  .with-navigation .brand > span {
    display: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  .brand-logo {
    transition: none;
  }
  .brand:hover .brand-logo {
    transform: none;
  }
}
</style>
