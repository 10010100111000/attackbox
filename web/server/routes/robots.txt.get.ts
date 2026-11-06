import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { markPublicStaticResponse } from '../utils/request-policy'
import { normalizeSiteOrigin } from '../../shared/seo'

export default defineEventHandler(event => {
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, getRequestURL(event).origin)
  markPublicStaticResponse(event)
  setResponseHeader(event, 'content-type', 'text/plain; charset=utf-8')
  setResponseHeader(event, 'cache-control', 'public, max-age=3600, s-maxage=3600')

  const allowPaths = [
    'Allow: /',
    'Allow: /learn',
    'Allow: /challenges',
    'Allow: /references',
    'Allow: /static-refs/',
    'Allow: /rss.xml',
    'Allow: /sitemap.xml',
    'Allow: /sitemap.txt'
  ]

  const disallowPaths = [
    'Disallow: /admin',
    'Disallow: /api/',
    'Disallow: /auth',
    'Disallow: /me',
    'Disallow: /notifications',
    'Disallow: /term',
    'Disallow: /gui',
    'Disallow: /discuss'
  ]

  const spiders = [
    '*',
    'Baiduspider',
    'Baiduspider-render',
    'Baiduspider-image',
    'Sogou web spider',
    'Sogou inst spider',
    '360Spider',
    'HaosouSpider',
    'Bytespider',
    'ToutiaoSpider',
    'YisouSpider',
    'Bingbot',
    'Googlebot'
  ]

  const lines: string[] = []
  for (const agent of spiders) {
    lines.push(`User-agent: ${agent}`, ...allowPaths, ...disallowPaths, '')
  }

  lines.push(`Sitemap: ${origin}/sitemap.xml`, `Sitemap: ${origin}/sitemap.txt`)

  return `${lines.join('\n')}\n`
})
