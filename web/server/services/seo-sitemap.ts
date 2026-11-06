import { asc, desc, eq } from 'drizzle-orm'
import { db } from '../db'
import {
  challenges,
  chapters,
  learningRoutes,
  modules,
  referenceDocuments
} from '../db/schema'
import { canonicalUrl, extractSeoExcerpt } from '../../shared/seo'

export interface SitemapEntry {
  loc: string
  lastmod?: string
  changefreq?: 'daily' | 'weekly' | 'monthly'
  priority?: string
  title?: string
  description?: string
}

function toIsoDate(value?: Date | string | null): string | undefined {
  if (!value) return undefined
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

export async function collectSitemapEntries(origin: string): Promise<{
  entries: SitemapEntry[]
  feedItems: SitemapEntry[]
}> {
  const entries: SitemapEntry[] = [
    { loc: canonicalUrl(origin, '/'), changefreq: 'daily', priority: '1.0' },
    { loc: canonicalUrl(origin, '/learn'), changefreq: 'daily', priority: '0.9' },
    { loc: `${canonicalUrl(origin, '/learn')}?tab=routes`, changefreq: 'weekly', priority: '0.8' },
    { loc: canonicalUrl(origin, '/challenges'), changefreq: 'weekly', priority: '0.8' },
    { loc: canonicalUrl(origin, '/references'), changefreq: 'weekly', priority: '0.8' }
  ]
  const feedItems: SitemapEntry[] = []

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
          title: modules.title,
          summary: modules.summary,
          updatedAt: modules.updatedAt,
          publishedAt: modules.publishedAt
        })
        .from(modules)
        .where(eq(modules.status, 'published'))
        .orderBy(desc(modules.publishedAt), desc(modules.id)),
      db
        .select({
          id: learningRoutes.id,
          title: learningRoutes.title,
          summary: learningRoutes.summary,
          updatedAt: learningRoutes.updatedAt
        })
        .from(learningRoutes)
        .where(eq(learningRoutes.status, 'published'))
        .orderBy(desc(learningRoutes.updatedAt)),
      db
        .select({
          id: challenges.id,
          title: challenges.title,
          summary: challenges.summary,
          bodyMarkdown: challenges.bodyMarkdown,
          updatedAt: challenges.updatedAt
        })
        .from(challenges)
        .where(eq(challenges.status, 'published'))
        .orderBy(desc(challenges.updatedAt)),
      db
        .select({
          slug: referenceDocuments.slug,
          title: referenceDocuments.title,
          summary: referenceDocuments.summary,
          bodyMarkdown: referenceDocuments.bodyMarkdown,
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
          title: chapters.title,
          position: chapters.position
        })
        .from(chapters)
        .where(eq(chapters.access, 'public'))
        .orderBy(asc(chapters.position))
    ])

    const moduleById = new Map(publishedModules.map(item => [item.id, item]))

    for (const mod of publishedModules) {
      const item: SitemapEntry = {
        loc: canonicalUrl(origin, `/learn/modules/${encodeURIComponent(mod.slug)}`),
        lastmod: toIsoDate(mod.updatedAt || mod.publishedAt),
        changefreq: 'weekly',
        priority: '0.8',
        title: `${mod.title} — 学习模块`,
        description: mod.summary || `在 AttackBox 学习《${mod.title}》网络安全实训模块。`
      }
      entries.push(item)
      feedItems.push(item)
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
        priority: '0.7',
        title: `${ch.title} — ${parentModule.title}`
      })
    }

    for (const route of publishedRoutes) {
      const item: SitemapEntry = {
        loc: canonicalUrl(origin, `/learn/routes/${encodeURIComponent(route.id)}`),
        lastmod: toIsoDate(route.updatedAt),
        changefreq: 'weekly',
        priority: '0.7',
        title: `${route.title} — 学习路线`,
        description: route.summary || `沿着《${route.title}》学习路线循序渐进掌握网络安全。`
      }
      entries.push(item)
      feedItems.push(item)
    }

    for (const challenge of publishedChallenges) {
      const item: SitemapEntry = {
        loc: canonicalUrl(origin, `/challenges/${encodeURIComponent(challenge.id)}`),
        lastmod: toIsoDate(challenge.updatedAt),
        changefreq: 'weekly',
        priority: '0.7',
        title: `${challenge.title} — 实战挑战`,
        description:
          challenge.summary ||
          extractSeoExcerpt(challenge.bodyMarkdown, 140) ||
          `在 AttackBox 参与《${challenge.title}》网络安全夺旗挑战。`
      }
      entries.push(item)
      feedItems.push(item)
    }

    for (const ref of publishedReferences) {
      const item: SitemapEntry = {
        loc: canonicalUrl(origin, `/references/${encodeURIComponent(ref.slug)}`),
        lastmod: toIsoDate(ref.updatedAt || ref.publishedAt),
        changefreq: 'monthly',
        priority: '0.7',
        title: `${ref.title} — 参考资料`,
        description:
          ref.summary ||
          extractSeoExcerpt(ref.bodyMarkdown, 140) ||
          `阅读《${ref.title}》网络安全参考资料。`
      }
      entries.push(item)
      feedItems.push(item)
    }
  } catch (error) {
    console.error('[sitemap] dynamic entries unavailable, serving static routes only:', error)
  }

  feedItems.sort((a, b) => (b.lastmod || '').localeCompare(a.lastmod || ''))
  return { entries, feedItems: feedItems.slice(0, 50) }
}
