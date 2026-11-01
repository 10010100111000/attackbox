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
/* 落地页外壳:只保留布局自身的骨架与页脚。
   首页各区块的样式分别位于 pages/index.vue 与 components/landing/*。 */
.landing-layout {
  min-height: 100vh;
  background: var(--bg-app);
  color: var(--text-main);
  overflow-x: clip;

  .wrap {
    width: min(1240px, calc(100% - 100px));
    margin: auto;
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
  }

  @media (max-width: 760px) {
    .wrap {
      width: calc(100% - 40px);
    }
    .platform-link {
      font-size: 10px;
      padding: 7px 10px;
      gap: 10px;
    }
    .footer {
      font-size: 10px;
    }
  }
}
</style>
