import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { markPublicStaticResponse } from '../utils/request-policy'
import { collectSitemapEntries } from '../services/seo-sitemap'
import { normalizeSiteOrigin } from '../../shared/seo'

export default defineEventHandler(async event => {
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, getRequestURL(event).origin)
  markPublicStaticResponse(event)
  setResponseHeader(event, 'content-type', 'text/plain; charset=utf-8')
  setResponseHeader(event, 'cache-control', 'public, max-age=1800, s-maxage=1800')

  const { entries } = await collectSitemapEntries(origin)
  return `${entries.map(entry => entry.loc).join('\n')}\n`
})
