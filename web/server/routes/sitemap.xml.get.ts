import { asc, desc, eq } from 'drizzle-orm'
import { defineEventHandler, getRequestURL, setResponseHeader } from 'h3'
import { db } from '../db'
import { markPublicStaticResponse } from '../utils/request-policy'
import {
  challenges,
  chapters,
  learningRoutes,
  modules,
  referenceDocuments
} from '../db/schema'
import { canonicalUrl, escapeXml, normalizeSiteOrigin } from '../../shared/seo'

interface SitemapEntry {
  loc: string
  lastmod?: string
  changefreq?: 'daily' | 'weekly' | 'monthly'
  priority?: string
}

function toIsoDate(value?: Date | string | null): string | undefined {
  if (!value) return undefined
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

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

  const entries: SitemapEntry[] = [
    { loc: canonicalUrl(origin, '/'), changefreq: 'daily', priority: '1.0' },
    { loc: canonicalUrl(origin, '/learn'), changefreq: 'daily', priority: '0.9' },
    { loc: canonicalUrl(origin, '/challenges'), changefreq: 'weekly', priority: '0.8' },
    { loc: canonicalUrl(origin, '/references'), changefreq: 'weekly', priority: '0.8' }
  ]

  try {
    const [
      publishedModules,
      publishedRoutes,
      publishedChallenges,
      publishedReferences,
      publicChapters
    ] = await Promise.all([
      db
        .select({
          id: modules.id,
          slug: modules.slug,
          updatedAt: modules.updatedAt,
          publishedAt: modules.publishedAt
        })
        .from(modules)
        .where(eq(modules.status, 'published'))
        .orderBy(desc(modules.publishedAt), desc(modules.id)),
      db
        .select({ id: learningRoutes.id, updatedAt: learningRoutes.updatedAt })
        .from(learningRoutes)
        .where(eq(learningRoutes.status, 'published'))
        .orderBy(desc(learningRoutes.updatedAt)),
      db
        .select({ id: challenges.id, updatedAt: challenges.updatedAt })
        .from(challenges)
        .where(eq(challenges.status, 'published'))
        .orderBy(desc(challenges.updatedAt)),
      db
        .select({
          slug: referenceDocuments.slug,
          updatedAt: referenceDocuments.updatedAt,
          publishedAt: referenceDocuments.publishedAt
        })
        .from(referenceDocuments)
        .where(eq(referenceDocuments.status, 'published'))
        .orderBy(desc(referenceDocuments.updatedAt)),
      db
        .select({
          moduleId: chapters.moduleId,
          slug: chapters.slug,
          position: chapters.position
        })
        .from(chapters)
        .where(eq(chapters.access, 'public'))
        .orderBy(asc(chapters.position))
    ])

    const moduleById = new Map(publishedModules.map(item => [item.id, item]))

    for (const mod of publishedModules) {
      entries.push({
        loc: canonicalUrl(origin, `/learn/modules/${encodeURIComponent(mod.slug)}`),
        lastmod: toIsoDate(mod.updatedAt || mod.publishedAt),
        changefreq: 'weekly',
        priority: '0.8'
      })
    }

    for (const ch of publicChapters) {
      const parentModule = moduleById.get(ch.moduleId)
      if (!parentModule) continue
      const chapterPath = `/learn/modules/${encodeURIComponent(
        parentModule.slug
      )}/${encodeURIComponent(ch.slug)}`
      entries.push({
        loc: canonicalUrl(origin, chapterPath),
        lastmod: toIsoDate(parentModule.updatedAt || parentModule.publishedAt),
        changefreq: 'weekly',
        priority: '0.7'
      })
    }

    for (const route of publishedRoutes) {
      entries.push({
        loc: canonicalUrl(origin, `/learn/routes/${encodeURIComponent(route.id)}`),
        lastmod: toIsoDate(route.updatedAt),
        changefreq: 'weekly',
        priority: '0.7'
      })
    }

    for (const challenge of publishedChallenges) {
      entries.push({
        loc: canonicalUrl(origin, `/challenges/${encodeURIComponent(challenge.id)}`),
        lastmod: toIsoDate(challenge.updatedAt),
        changefreq: 'weekly',
        priority: '0.7'
      })
    }

    for (const ref of publishedReferences) {
      entries.push({
        loc: canonicalUrl(origin, `/references/${encodeURIComponent(ref.slug)}`),
        lastmod: toIsoDate(ref.updatedAt || ref.publishedAt),
        changefreq: 'monthly',
        priority: '0.7'
      })
    }
  } catch (error) {
    // Fallback to core static routes if database is temporarily unreachable.
    console.error('[sitemap] dynamic entries unavailable, serving static routes only:', error)
  }

  const xmlHeader = '\x3c?xml version="1.0" encoding="UTF-8"?\x3e'
  const urlsetOpen = '\x3curlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"\x3e'
  const urlsetClose = '\x3c/urlset\x3e'
  return [xmlHeader, urlsetOpen, ...entries.map(renderUrlNode), urlsetClose, ''].join('\n')
})
