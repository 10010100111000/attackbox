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
  background: var(--bg);
  color: var(--ink);
  overflow-x: clip;

  .wrap {
    width: min(1240px, calc(100% - 100px));
    margin: auto;
  }

  .platform-link {
    border: 1px solid var(--line-strong);
    background: rgba(255, 255, 255, 0.03);
    box-shadow: var(--surface-highlight);
    padding: 7px 14px;
    border-radius: var(--radius-md, 7px);
    display: flex;
    align-items: center;
    gap: 18px;
    font-size: 12px;
    font-weight: 550;
    color: var(--ink);
    text-decoration: none;
    transition:
      background 0.18s ease,
      border-color 0.18s ease,
      transform 0.18s ease;
  }

  .platform-link:hover {
    background: rgba(255, 255, 255, 0.08);
    border-color: var(--brand-border);
    transform: translateY(-1px);
  }

  .footer {
    border-top: 1px solid var(--line);
    padding: 28px 0 34px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: var(--muted);
    font-size: 11px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .footer-brand {
    color: var(--ink);
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-weight: 600;
    font-size: 13px;
    text-decoration: none;
  }

  @media (max-width: 1100px) {
    .wrap {
      width: calc(100% - 60px);
    }
  }

  @media (max-width: 760px) {
    .wrap {
      width: calc(100% - 40px);
    }
    .platform-link {
      font-size: 11px;
      padding: 6px 11px;
      gap: 10px;
    }
    .footer {
      font-size: 10px;
    }
  }
}
</style>
