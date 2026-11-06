import { useHead, useRequestURL } from '#imports'
import { buildVerificationMeta, canonicalUrl, normalizeSiteOrigin } from '~/shared/seo'

export default defineNuxtPlugin(() => {
  const reqUrl = useRequestURL()
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, reqUrl.origin)
  const currentCanonical = canonicalUrl(origin, reqUrl.pathname)

  const verificationMetas = buildVerificationMeta({
    baidu: process.env.BAIDU_SITE_VERIFICATION,
    bing: process.env.BING_SITE_VERIFICATION,
    google: process.env.GOOGLE_SITE_VERIFICATION,
    sogou: process.env.SOGOU_SITE_VERIFICATION,
    so360: process.env.SO360_SITE_VERIFICATION,
    shenma: process.env.SHENMA_SITE_VERIFICATION,
    bytedance: process.env.BYTEDANCE_SITE_VERIFICATION
  })

  useHead({
    meta: [
      {
        name: 'mobile-agent',
        content: `format=html5; url=${currentCanonical}`
      },
      ...verificationMetas
    ]
  })
})
