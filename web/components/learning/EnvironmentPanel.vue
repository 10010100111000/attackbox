<template>
  <section class="environment-panel" aria-label="实验环境">
    <header>
      <h2>实验环境</h2>
      <div class="environment-actions"><slot name="controls" /></div>
    </header>
    <slot />
  </section>
</template>
<style scoped>
.environment-panel {
  margin-block: 28px;
  padding: 16px 18px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
  color: var(--ink);
}
header,
.environment-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
header {
  justify-content: space-between;
  gap: 12px;
}
h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.environment-actions {
  justify-content: flex-end;
}
</style>
