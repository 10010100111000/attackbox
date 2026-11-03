<script setup lang="ts">
const { currentUser } = useAuth()
</script>
<template>
  <div class="landing-layout brand-experience" data-theme="midnight">
    <SiteHeader>
      <template #actions>
        <UserMenu v-if="currentUser" />
        <NuxtLink v-else to="/learn" class="platform-link">进入平台</NuxtLink>
      </template>
    </SiteHeader>

    <main>
      <slot />
    </main>

    <div class="wrap">
      <footer class="footer">
        <NuxtLink to="/" class="footer-brand"> AttackBox </NuxtLink>
        <span class="copyright">© 2026 AttackBox</span>
      </footer>
    </div>
  </div>
</template>

<style>
.landing-layout {
  min-height: 100vh;
  background-color: #09090b;
  background: var(--bg, #09090b);
  color: var(--ink, #fafafa);
  overflow-x: clip;
}

.landing-layout .wrap {
  width: min(1240px, calc(100% - 100px));
  margin: auto;
}

.landing-layout .platform-link {
  border: 1px solid var(--line-strong);
  background: rgba(255, 255, 255, 0.03);
  box-shadow: var(--surface-highlight);
  padding: 7px 14px;
  border-radius: var(--radius-md, 7px);
  display: flex;
  align-items: center;
  gap: 18px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
  text-decoration: none;
  transition:
    background 0.18s ease,
    border-color 0.18s ease,
    transform 0.18s ease;
}

.landing-layout .platform-link:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: var(--brand-border);
  transform: translateY(-1px);
}

.landing-layout .footer {
  border-top: 1px solid var(--line);
  padding: 28px 0 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--muted);
  font-size: 11px;
  font-family: var(--font-mono);
}

.landing-layout .footer-brand {
  color: var(--ink);
  font-family: var(--font-sans);
  font-weight: 700;
  font-size: 13px;
  text-decoration: none;
}

@media (max-width: 1100px) {
  .landing-layout .wrap {
    width: calc(100% - 60px);
  }
}

@media (max-width: 760px) {
  .landing-layout .wrap {
    width: calc(100% - 40px);
  }
  .landing-layout .platform-link {
    font-size: 11px;
    padding: 6px 11px;
    gap: 10px;
  }
  .landing-layout .footer {
    font-size: 10px;
  }
}
</style>
