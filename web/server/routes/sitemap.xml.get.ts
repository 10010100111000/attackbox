import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { markPublicStaticResponse } from '../utils/request-policy'
import { collectSitemapEntries, type SitemapEntry } from '../services/seo-sitemap'
import { escapeXml, normalizeSiteOrigin } from '../../shared/seo'

function renderUrlNode(entry: SitemapEntry): string {
  const parts = [`    \x3cloc\x3e${escapeXml(entry.loc)}\x3c/loc\x3e`]
  if (entry.lastmod) {
    parts.push(`    \x3clastmod\x3e${escapeXml(entry.lastmod)}\x3c/lastmod\x3e`)
  }
  if (entry.changefreq) {
    parts.push(`    \x3cchangefreq\x3e${entry.changefreq}\x3c/changefreq\x3e`)
  }
  if (entry.priority) {
    parts.push(`    \x3cpriority\x3e${entry.priority}\x3c/priority\x3e`)
  }
  return `  \x3curl\x3e\n${parts.join('\n')}\n  \x3c/url\x3e`
}

export default defineEventHandler(async event => {
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, getRequestURL(event).origin)
  markPublicStaticResponse(event)
  setResponseHeader(event, 'content-type', 'application/xml; charset=utf-8')
  setResponseHeader(event, 'cache-control', 'public, max-age=1800, s-maxage=1800')

  const { entries } = await collectSitemapEntries(origin)
  const xmlHeader = '\x3c?xml version="1.0" encoding="UTF-8"?\x3e'
  const urlsetOpen = '\x3curlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"\x3e'
  const urlsetClose = '\x3c/urlset\x3e'
  return [xmlHeader, urlsetOpen, ...entries.map(renderUrlNode), urlsetClose, ''].join('\n')
})
