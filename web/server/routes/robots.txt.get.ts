import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { markPublicStaticResponse } from '../utils/request-policy'
import { normalizeSiteOrigin } from '../../shared/seo'

export default defineEventHandler(event => {
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, getRequestURL(event).origin)
  markPublicStaticResponse(event)
  setResponseHeader(event, 'content-type', 'text/plain; charset=utf-8')
  setResponseHeader(event, 'cache-control', 'public, max-age=3600, s-maxage=3600')

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

  const lines = [
    'User-agent: *',
    'Allow: /',
    'Allow: /learn',
    'Allow: /challenges',
    'Allow: /references',
    ...disallowPaths,
    '',
    'User-agent: Baiduspider',
    'Allow: /',
    'Allow: /learn',
    'Allow: /challenges',
    'Allow: /references',
    ...disallowPaths,
    '',
    'User-agent: Sogou web spider',
    'Allow: /',
    ...disallowPaths,
    '',
    'User-agent: 360Spider',
    'Allow: /',
    ...disallowPaths,
    '',
    'User-agent: Bytespider',
    'Allow: /',
    ...disallowPaths,
    '',
    'User-agent: YisouSpider',
    'Allow: /',
    ...disallowPaths,
    '',
    `Sitemap: ${origin}/sitemap.xml`
  ]

  return `${lines.join('\n')}\n`
})
