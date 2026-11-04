import { DEFAULT_SEO_KEYWORDS } from './shared/seo'

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
    '/auth': {
      prerender: true,
      headers: { 'X-Robots-Tag': 'noindex, follow' }
    },
    '/admin/**': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/me': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/notifications': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/term': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/gui': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/discuss': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } }
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
        {
          name: 'description',
          content:
            '理解原理，亲手验证。在 AttackBox 学习网络安全、Linux 与 Web 安全教程，在真实容器沙箱与靶场环境中动手实践。'
        },
        { name: 'keywords', content: DEFAULT_SEO_KEYWORDS },
        { name: 'applicable-device', content: 'pc,mobile' },
        { name: 'format-detection', content: 'telephone=no, email=no' },
        { property: 'og:locale', content: 'zh_CN' },
        { property: 'og:site_name', content: 'AttackBox' },
        // 暂无站点分享封面图;配置 og:image 后可改回 summary_large_image
        { name: 'twitter:card', content: 'summary' }
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
