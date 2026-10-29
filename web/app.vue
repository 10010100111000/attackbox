<template>
  <div
    class="min-h-screen bg-background text-foreground antialiased selection:bg-accent/25 selection:text-foreground"
  >
    <NuxtLoadingIndicator :height="2" :throttle="180" color="var(--learning-spark, #bd7951)" />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <UiConfirmDialog />
  </div>
</template>
