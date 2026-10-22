<script setup lang="ts">
import HeroSection from '~/components/landing/HeroSection.vue'
import FeaturedModule from '~/components/landing/FeaturedModule.vue'
import ModuleRail from '~/components/landing/ModuleRail.vue'

definePageMeta({
  layout: 'landing'
})
</script>

<template>
  <div>
    <HeroSection />
    <section id="modules" class="modules wrap">
      <div class="section-head">
        <h2>下一步，动手。</h2>
      </div>
      <FeaturedModule />
      <ModuleRail />
    </section>
  </div>
</template>
