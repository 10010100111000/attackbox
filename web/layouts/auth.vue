<template>
  <div class="auth-layout brand-experience">
    <SiteHeader />
    <div class="shell">
      <slot />

      <footer class="footer">
        <span>© 2026 AttackBox</span>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.auth-layout {
  min-height: 100svh;
  display: flex;
  flex-direction: column;
  background:
    radial-gradient(ellipse at 18% 40%, var(--brand-warm-soft), transparent 55%), var(--bg-app);
  color: var(--text-main);
}

.shell {
  flex: 1;
  width: 100%;
  min-height: 0;
  max-width: 1600px;
  margin: auto;
  display: flex;
  flex-direction: column;
  padding: 0 48px;
}

.footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 48px;
  color: var(--text-muted);
  font-size: 9px;
  flex-shrink: 0;
}

@media (max-width: 1100px) {
  .shell {
    padding: 0 30px;
  }
}

@media (max-width: 760px) {
  .shell {
    padding: 0 25px;
  }
  .footer {
    height: 48px;
    font-size: 8px;
  }
}
</style>

<style src="~/assets/css/auth-forms.css"></style>
