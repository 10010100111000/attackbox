// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: false },
  modules: ['@nuxtjs/tailwindcss', 'reka-ui/nuxt', '@nuxt/eslint'],
  tailwindcss: {
    cssPath: '~/assets/css/main.css'
  },
  typescript: {
    strict: true
  },
  css: ['~/assets/css/animations.css', '~/assets/css/brand-experience.css'],
  hooks: {
    'build:manifest': manifest => {
      // Cloudflare can reject Sec-Purpose: prefetch with 503. Keep required
      // preloads and client route imports, without speculative build hints.
      for (const resource of Object.values(manifest)) resource.prefetch = false
    }
  },
  routeRules: {
    '/': { prerender: false },
    '/auth': { prerender: true }
  },
  nitro: {
    experimental: { websocket: true },
    prerender: {
      crawlLinks: false,
      routes: ['/auth']
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
        { rel: 'apple-touch-icon', sizes: '180x180', href: '/apple-touch-icon.png' }
      ],
      // 首屏前置读取本地主题,避免 SSR 默认「深海」在 hydration 后跳变成「石墨」造成闪烁
      script: [
        {
          type: 'text/javascript',
          innerHTML: `(function(){try{var t=localStorage.getItem("attackbox-appearance");if(t!=="graphite"&&t!=="midnight"){t="graphite"}document.documentElement.setAttribute("data-theme",t)}catch(e){document.documentElement.setAttribute("data-theme","graphite")}})();`
        }
      ]
    }
  }
})
