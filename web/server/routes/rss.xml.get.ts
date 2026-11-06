import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { markPublicStaticResponse } from '../utils/request-policy'
import { collectSitemapEntries } from '../services/seo-sitemap'
import {
  DEFAULT_SEO_DESCRIPTION,
  SITE_NAME,
  canonicalUrl,
  escapeXml,
  normalizeSiteOrigin
} from '../../shared/seo'

export default defineEventHandler(async event => {
  const origin = normalizeSiteOrigin(process.env.APP_BASE_URL, getRequestURL(event).origin)
  markPublicStaticResponse(event)
  setResponseHeader(event, 'content-type', 'application/rss+xml; charset=utf-8')
  setResponseHeader(event, 'cache-control', 'public, max-age=1800, s-maxage=1800')

  const { feedItems } = await collectSitemapEntries(origin)
  const homeUrl = canonicalUrl(origin, '/')
  const feedUrl = canonicalUrl(origin, '/rss.xml')

  const itemsXml = feedItems.map(item => {
    const pubDate = item.lastmod ? new Date(item.lastmod).toUTCString() : new Date().toUTCString()
    return [
      '    \x3citem\x3e',
      `      \x3ctitle\x3e${escapeXml(item.title || SITE_NAME)}\x3c/title\x3e`,
      `      \x3clink\x3e${escapeXml(item.loc)}\x3c/link\x3e`,
      `      \x3cguid isPermaLink="true"\x3e${escapeXml(item.loc)}\x3c/guid\x3e`,
      `      \x3cdescription\x3e${escapeXml(item.description || DEFAULT_SEO_DESCRIPTION)}\x3c/description\x3e`,
      `      \x3cpubDate\x3e${escapeXml(pubDate)}\x3c/pubDate\x3e`,
      '    \x3c/item\x3e'
    ].join('\n')
  })

  return [
    '\x3c?xml version="1.0" encoding="UTF-8"?\x3e',
    '\x3crss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"\x3e',
    '  \x3cchannel\x3e',
    `    \x3ctitle\x3e${escapeXml(SITE_NAME)} — 网络安全学习与实战靶场\x3c/title\x3e`,
    `    \x3clink\x3e${escapeXml(homeUrl)}\x3c/link\x3e`,
    `    \x3cdescription\x3e${escapeXml(DEFAULT_SEO_DESCRIPTION)}\x3c/description\x3e`,
    '    \x3clanguage\x3ezh-CN\x3c/language\x3e',
    `    \x3catom:link href="${escapeXml(feedUrl)}" rel="self" type="application/rss+xml" /\x3e`,
    ...itemsXml,
    '  \x3c/channel\x3e',
    '\x3c/rss\x3e',
    ''
  ].join('\n')
})
