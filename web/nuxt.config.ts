// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: false },
  modules: [
    '@nuxtjs/tailwindcss',
    'reka-ui/nuxt'
  ],
  tailwindcss: {
    cssPath: '~/assets/css/main.css'
  },
  typescript: {
    strict: true
  },
  css: [
    '~/assets/css/animations.css'
  ],
  routeRules: {
    '/': { prerender: true },
    '/auth': { prerender: true }
  },
  nitro: {
    prerender: {
      crawlLinks: false,
      routes: ['/', '/auth']
    }
  },
  app: {
    head: {
      htmlAttrs: {
        lang: 'zh-CN'
      },
      title: 'AttackBox — 理解原理，亲手验证。',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: '理解原理，亲手验证。在 AttackBox 探索安全学习与实践。' }
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
        { rel: 'apple-touch-icon', href: '/logo.png' }
      ]
    }
  }
})
