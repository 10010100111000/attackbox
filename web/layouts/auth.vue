<template>
  <div class="auth-layout brand-experience">
    <div class="shell">
      <header class="header">
        <NuxtLink to="/" class="brand">
          <img src="/logo.png" alt="AttackBox Logo" class="brand-logo" width="31" height="31" />
          <span>AttackBox</span>
        </NuxtLink>
        <UserMenu/>
      </header>

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
  background: radial-gradient(ellipse at 18% 40%, var(--brand-warm-soft), transparent 55%), var(--bg-app);
  color: var(--text-main);
}

.shell {
  min-height: 100svh;
  max-width: 1600px;
  margin: auto;
  display: flex;
  flex-direction: column;
  padding: 0 48px;
}

.header {
  height: 96px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 19px;
  font-weight: 650;
  letter-spacing: -0.6px;
  color: inherit;
  text-decoration: none;
}

.brand-logo {
  width: 31px;
  height: 31px;
  border-radius: 8px;
  object-fit: contain;
  display: block;
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
    min-height: 100svh;
  }

  .header {
    height: 78px;
  }

  .brand {
    font-size: 17px;
    gap: 8px;
  }

  .brand-logo {
    width: 28px;
    height: 28px;
    font-size: 19px;
    border-radius: 7px;
  }

  .footer {
    height: 48px;
    font-size: 8px;
  }

}

@media (max-height: 700px) and (min-width: 761px) {
  .header {
    height: 78px;
  }
}
</style>
