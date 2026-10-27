import path from 'node:path'
import { and, eq, sql, inArray } from 'drizzle-orm'
import { createError } from 'h3'
import MarkdownIt from 'markdown-it'
import { db } from '../db'
import {
  modules,
  chapters,
  chapterQuestions,
  contentCategories,
  moduleSources,
  chapterSources,
  questionSources,
  assets,
  chapterAssets,
  moduleImages,
} from '../db/schema'
import { ContentFormatError, parseModuleMarkdown, contentMatchKey } from '../content/parse-module'
import type { ContentImage } from '../content/parse-module'
import { hash, storeImage } from '../content/storage'
import { fetchGithubTree, fetchGithubBlob } from '../content/github'
import type { RepositoryConfig, GithubTreeEntry } from '../content/github'
import { pruneUnusedCategories, saveModuleInTransaction } from './content'
import { slugFor } from './content-input'
import type { ModuleInput } from '../../shared/content'
import type { RepoModuleEntry, RepoScanReport, SyncModuleResult } from '../../shared/content-sync'

type Tx = Parameters<Parameters<typeof db.transaction>[0]>[0]

interface ModuleGitManifest {
  md: string
  images: Record<string, string>
}

const allowedImages = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif', '.avif'])
const mediaTypes: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.gif': 'image/gif',
  '.avif': 'image/avif',
}

function parseManifest(raw: string | null | undefined): ModuleGitManifest | null {
  if (!raw || !raw.startsWith('{')) return null
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed.md === 'string' && parsed.images && typeof parsed.images === 'object') {
      return parsed as ModuleGitManifest
    }
  } catch {
    // Legacy hash string; treat as needing differential refresh
  }
  return null
}

function originKey(config: RepositoryConfig) {
  return `github:${config.repository.toLowerCase()}:${config.directory}`
}

function resolveTreeImageEntry(
  mdPath: string,
  image: ContentImage,
  blobMap: Map<string, GithubTreeEntry>,
): GithubTreeEntry {
  let href: string
  try {
    href = decodeURIComponent(image.href)
  } catch {
    throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message: '图片地址编码无效' }])
  }
  if (!href || href.includes('\0') || href.includes('\\') || /[:?#]/.test(href) || path.posix.isAbsolute(href)) {
    throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message: '图片必须使用仓库内相对路径' }])
  }
  const baseDir = path.posix.dirname(mdPath)
  const resolved = path.posix.normalize(baseDir === '.' ? href : `${baseDir}/${href}`)
  if (resolved === '..' || resolved.startsWith('../') || path.posix.isAbsolute(resolved)) {
    throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message: '图片路径越过仓库根目录' }])
  }
  const ext = path.posix.extname(resolved).toLowerCase()
  if (!allowedImages.has(ext)) {
    throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message: '仅支持 PNG、JPEG、WebP、GIF 或 AVIF 图片' }])
  }
  const entry = blobMap.get(resolved)
  if (!entry || entry.type !== 'blob' || !entry.size || entry.size <= 0 || entry.size > 10 * 1024 * 1024) {
    throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message: `仓库中找不到图片或文件大小无效 (${resolved})` }])
  }
  return entry
}

async function ensureCategoryId(tx: Tx, categories: (typeof contentCategories.$inferSelect)[], key: string | null) {
  if (!key) return null
  const existing = categories.find(c => c.slug === key || c.name === key) || null
  if (existing) return existing.id
  const candidate =
    /^[a-z0-9]+(-[a-z0-9]+)*$/.test(key) && key.length <= 100 && !categories.some(c => c.slug === key)
      ? key
      : slugFor(undefined, 'category')
  const [created] = await tx.insert(contentCategories).values({ name: key, slug: candidate }).returning()
  categories.push(created!)
  return created!.id
}

function discoverMarkdownEntries(entries: GithubTreeEntry[]): GithubTreeEntry[] {
  return entries
    .filter(e => {
      if (e.type !== 'blob' || !e.path.endsWith('.md')) return false
      const parts = e.path.split('/')
      if (parts.some(p => p.startsWith('.'))) return false
      const base = parts[parts.length - 1]!
      return base.toLowerCase() !== 'readme.md'
    })
    .sort((a, b) => a.path.localeCompare(b.path))
}

/** Reads Git tree metadata and computes live diff status for every module without downloading file contents. */
export async function scanRepositoryModules(config: RepositoryConfig, token: string): Promise<RepoScanReport> {
  const { commit, entries } = await fetchGithubTree(config, token)
  const blobMap = new Map(entries.filter(e => e.type === 'blob').map(e => [e.path, e]))
  const mdEntries = discoverMarkdownEntries(entries)
  if (!mdEntries.length) {
    throw createError({ statusCode: 400, message: '仓库指定目录中没有找到模块 .md 文件' })
  }

  const [mods, sources] = await Promise.all([
    db.select().from(modules),
    db.select().from(moduleSources),
  ])

  const repoModules: RepoModuleEntry[] = mdEntries.map(mdEntry => {
    const title = path.posix.basename(mdEntry.path, '.md').trim()
    const folder = path.posix.dirname(mdEntry.path) === '.' ? '根目录' : path.posix.dirname(mdEntry.path)
    const matchedSource =
      sources.find(s => s.sourcePath === mdEntry.path) ||
      sources.find(s => path.posix.basename(s.sourcePath, '.md').trim() === title)
    const matchedMod =
      (matchedSource ? mods.find(m => m.id === matchedSource.moduleId) : undefined) ||
      mods.find(m => m.title.normalize('NFKC') === title.normalize('NFKC'))

    if (!matchedMod) {
      return {
        sourcePath: mdEntry.path,
        folder,
        title,
        mdSha: mdEntry.sha,
        status: 'new',
        changedReasons: ['新模块（尚未导入）'],
        moduleId: null,
        moduleSlug: null,
        moduleStatus: null,
        syncedCommit: null,
        syncedAt: null,
      }
    }

    const manifest = parseManifest(matchedSource?.sourceHash)
    const changedReasons: string[] = []
    if (!manifest) {
      changedReasons.push('首次启用 Git 差异清单同步')
    } else {
      if (manifest.md !== mdEntry.sha) {
        changedReasons.push('Markdown 正文或配置头已修改')
      }
      if (matchedSource?.sourcePath !== mdEntry.path) {
        changedReasons.push(`模块路径变更 (${matchedSource?.sourcePath} -> ${mdEntry.path})`)
      }
      let changedImages = 0
      for (const [href, oldSha] of Object.entries(manifest.images)) {
        const baseDir = path.posix.dirname(mdEntry.path)
        const resolved = path.posix.normalize(baseDir === '.' ? href : `${baseDir}/${href}`)
        const currentBlob = blobMap.get(resolved)
        if (!currentBlob || currentBlob.sha !== oldSha) {
          changedImages++
        }
      }
      if (changedImages > 0) {
        changedReasons.push(`${changedImages} 张引用图片有更新`)
      }
    }

    return {
      sourcePath: mdEntry.path,
      folder,
      title,
      mdSha: mdEntry.sha,
      status: changedReasons.length > 0 ? 'modified' : 'up_to_date',
      changedReasons,
      moduleId: matchedMod.id,
      moduleSlug: matchedMod.slug,
      moduleStatus: matchedMod.status,
      syncedCommit: matchedSource?.commit || null,
      syncedAt: matchedSource?.syncedAt ? matchedSource.syncedAt.toISOString() : null,
    }
  })

  return {
    commit,
    repository: config.repository,
    branch: config.branch,
    directory: config.directory,
    modules: repoModules,
  }
}

/** Syncs one specific module (or all new/modified modules) by fetching ONLY changed blobs. */
export async function syncRepositoryModules(
  config: RepositoryConfig,
  token: string,
  actor: string,
  options: { sourcePath?: string; moduleId?: string; force?: boolean },
): Promise<SyncModuleResult> {
  const { commit, entries } = await fetchGithubTree(config, token)
  const blobMap = new Map(entries.filter(e => e.type === 'blob').map(e => [e.path, e]))
  const mdEntries = discoverMarkdownEntries(entries)
  if (!mdEntries.length) {
    throw createError({ statusCode: 400, message: '仓库指定目录中没有找到模块 .md 文件' })
  }

  const [allMods, allSources] = await Promise.all([
    db.select().from(modules),
    db.select().from(moduleSources),
  ])

  let targets: GithubTreeEntry[] = []
  if (options.sourcePath) {
    const target = mdEntries.find(e => e.path === options.sourcePath || path.posix.dirname(e.path) === options.sourcePath)
    if (!target) throw createError({ statusCode: 404, message: `仓库中未找到模块: ${options.sourcePath}` })
    targets = [target]
  } else if (options.moduleId) {
    const mod = allMods.find(m => m.id === options.moduleId)
    if (!mod) throw createError({ statusCode: 404, message: '模块不存在' })
    const src = allSources.find(s => s.moduleId === mod.id)
    const target =
      (src ? mdEntries.find(e => e.path === src.sourcePath) : undefined) ||
      mdEntries.find(e => path.posix.basename(e.path, '.md').trim().normalize('NFKC') === mod.title.normalize('NFKC'))
    if (!target) throw createError({ statusCode: 404, message: `Git 仓库中未找到与“${mod.title}”对应的 .md 文件` })
    targets = [target]
  } else {
    // Sync all new or modified modules
    const report = await scanRepositoryModules(config, token)
    const changedPaths = new Set(
      report.modules.filter(m => options.force || m.status !== 'up_to_date').map(m => m.sourcePath),
    )
    targets = mdEntries.filter(e => changedPaths.has(e.path))
  }

  const synced: SyncModuleResult['synced'] = []
  const origin = originKey(config)

  for (const mdEntry of targets) {
    if ((mdEntry.size || 0) > 2 * 1024 * 1024) {
      throw createError({ statusCode: 400, message: `${mdEntry.path}: 单个教程不能超过 2 MiB` })
    }

    const title = path.posix.basename(mdEntry.path, '.md').trim()
    const matchedSource =
      allSources.find(s => s.sourcePath === mdEntry.path) ||
      allSources.find(s => path.posix.basename(s.sourcePath, '.md').trim() === title)
    const existingMod =
      (matchedSource ? allMods.find(m => m.id === matchedSource.moduleId) : undefined) ||
      allMods.find(m => m.title.normalize('NFKC') === title.normalize('NFKC'))

    const oldManifest = parseManifest(matchedSource?.sourceHash)

    // Download only the .md file
    const rawMd = (await fetchGithubBlob(config, token, mdEntry.sha, mdEntry.size)).toString('utf8')
    const parsed = parseModuleMarkdown(mdEntry.path, rawMd)

    // Resolve all referenced images against the Git tree metadata
    const uniqueImages = new Map<string, GithubTreeEntry>()
    for (const img of parsed.images) {
      if (!uniqueImages.has(img.href)) {
        uniqueImages.set(img.href, resolveTreeImageEntry(mdEntry.path, img, blobMap))
      }
    }

    const newManifest: ModuleGitManifest = {
      md: mdEntry.sha,
      images: Object.fromEntries([...uniqueImages.entries()].map(([href, entry]) => [href, entry.sha])),
    }

    // Check existing module_images in DB so unchanged images don't need to be downloaded from GitHub
    const existingImgRows = existingMod
      ? await db.select().from(moduleImages).where(eq(moduleImages.moduleId, existingMod.id))
      : []
    const existingHrefToAssetId = new Map(existingImgRows.map(r => [r.href, r.assetId]))

    if (
      !options.force &&
      existingMod &&
      oldManifest &&
      oldManifest.md === newManifest.md &&
      matchedSource?.sourcePath === mdEntry.path &&
      Object.keys(newManifest.images).length === Object.keys(oldManifest.images).length &&
      Object.entries(newManifest.images).every(([href, sha]) => oldManifest.images[href] === sha && existingHrefToAssetId.has(href))
    ) {
      await db
        .update(moduleSources)
        .set({ commit, syncedAt: new Date() })
        .where(eq(moduleSources.moduleId, existingMod.id))
      synced.push({
        moduleId: existingMod.id,
        title: existingMod.title,
        sourcePath: mdEntry.path,
        action: 'unchanged',
        downloadedImages: 0,
        reusedImages: uniqueImages.size,
      })
      continue
    }

    // Differential image download: ONLY download images whose Git blob SHA changed or isn't bound yet
    const downloadedBlobs = new Map<string, { bytes: Buffer; sha256Hex: string; mediaType: string }>()
    let downloadedImages = 0
    let reusedImages = 0

    for (const [href, imgEntry] of uniqueImages.entries()) {
      const canReuse =
        oldManifest?.images?.[href] === imgEntry.sha &&
        existingHrefToAssetId.has(href)
      if (canReuse) {
        reusedImages++
        continue
      }
      if (!downloadedBlobs.has(imgEntry.sha)) {
        const bytes = await fetchGithubBlob(config, token, imgEntry.sha, imgEntry.size)
        const sha256Hex = await storeImage(bytes)
        const ext = path.posix.extname(imgEntry.path).toLowerCase()
        downloadedBlobs.set(imgEntry.sha, {
          bytes,
          sha256Hex,
          mediaType: mediaTypes[ext]!,
        })
        downloadedImages++
      } else {
        reusedImages++
      }
    }

    // Write changes directly in a transaction
    const savedId = await db.transaction(async tx => {
      await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
      const categories = await tx.select().from(contentCategories)
      const categoryId = await ensureCategoryId(tx, categories, parsed.categorySlug)

      const existingChapters = existingMod
        ? await tx.select().from(chapters).where(eq(chapters.moduleId, existingMod.id))
        : []
      const existingChSources = existingMod
        ? await tx.select().from(chapterSources).where(eq(chapterSources.moduleId, existingMod.id))
        : []
      const existingQuestions = existingChapters.length
        ? await tx
            .select()
            .from(chapterQuestions)
            .where(inArray(chapterQuestions.chapterId, existingChapters.map(c => c.id)))
        : []
      const existingQSources = existingChapters.length
        ? await tx
            .select()
            .from(questionSources)
            .where(inArray(questionSources.chapterId, existingChapters.map(c => c.id)))
        : []

      const usedChapterIds = new Set<string>()
      const chapterInputs = parsed.chapters.map(ch => {
        const linkedChId = existingChSources.find(cs => cs.matchKey === ch.matchKey)?.chapterId
        const matchedCh =
          (linkedChId ? existingChapters.find(c => c.id === linkedChId) : undefined) ||
          existingChapters.find(c => contentMatchKey(c.title) === ch.matchKey && !usedChapterIds.has(c.id))
        if (matchedCh) usedChapterIds.add(matchedCh.id)

        const usedQuestionIds = new Set<string>()
        const questionInputs = ch.questions.map(q => {
          const linkedQId = matchedCh
            ? existingQSources.find(qs => qs.chapterId === matchedCh.id && qs.matchKey === q.matchKey)?.questionId
            : undefined
          const matchedQ = matchedCh
            ? (linkedQId ? existingQuestions.find(eqQ => eqQ.id === linkedQId) : undefined) ||
              existingQuestions.find(
                eqQ =>
                  eqQ.chapterId === matchedCh.id &&
                  contentMatchKey(eqQ.promptMarkdown) === q.matchKey &&
                  !usedQuestionIds.has(eqQ.id),
              )
            : undefined
          if (matchedQ) usedQuestionIds.add(matchedQ.id)
          return {
            id: matchedQ?.id,
            promptMarkdown: q.promptMarkdown,
            answer: q.answer,
            normalization: q.normalization,
            errorHint: q.errorHint,
          }
        })

        return {
          chapterId: matchedCh?.id,
          title: ch.title,
          access: ch.access,
          durationMinutes: ch.durationMinutes,
          bodyMarkdown: ch.bodyMarkdown,
          questions: questionInputs,
        }
      })

      const input: ModuleInput = {
        title: parsed.title,
        summary: parsed.summary,
        descriptionMarkdown: parsed.descriptionMarkdown,
        learningOutcomes: parsed.learningOutcomes,
        difficulty: parsed.difficulty,
        categoryId,
        lockVersion: existingMod?.lockVersion,
        chapters: chapterInputs,
      }

      const saved = await saveModuleInTransaction(tx, existingMod?.id || null, actor, input)
      const moduleId = saved.module.id

      const storedChapters = await tx.select().from(chapters).where(eq(chapters.moduleId, moduleId))
      await tx.delete(chapterSources).where(eq(chapterSources.moduleId, moduleId))
      await tx.delete(moduleImages).where(eq(moduleImages.moduleId, moduleId))

      const assetMap = new Map<string, string>()
      const descriptionImages = new MarkdownIt({ html: false })
        .parse(parsed.descriptionMarkdown, {})
        .flatMap(t => (t.children || []).filter(c => c.type === 'image').map(c => String(c.attrGet('src'))))

      for (const [href, imgEntry] of uniqueImages.entries()) {
        let assetId =
          oldManifest?.images?.[href] === imgEntry.sha ? existingHrefToAssetId.get(href) : undefined
        if (!assetId) {
          const blob = downloadedBlobs.get(imgEntry.sha)!
          let [asset] = await tx.select().from(assets).where(eq(assets.storageKey, blob.sha256Hex))
          if (!asset) {
            ;[asset] = await tx
              .insert(assets)
              .values({
                storageKey: blob.sha256Hex,
                originalName: path.posix.basename(imgEntry.path),
                mediaType: blob.mediaType,
                byteSize: blob.bytes.length,
                sha256: Buffer.from(blob.sha256Hex, 'hex'),
                visibility: 'private',
                uploadedBy: actor,
              })
              .returning()
          }
          assetId = asset!.id
        }
        assetMap.set(href, assetId)
        await tx.insert(moduleImages).values({
          moduleId,
          href,
          assetId,
          public: descriptionImages.includes(href) || href === parsed.cover,
        })
      }

      await tx
        .update(modules)
        .set({ coverAssetId: parsed.cover ? assetMap.get(parsed.cover)! : null })
        .where(eq(modules.id, moduleId))

      for (const ch of parsed.chapters) {
        const stored = storedChapters.find(s => s.position === ch.position)!
        await tx.insert(chapterSources).values({ chapterId: stored.id, moduleId, matchKey: ch.matchKey })
        await tx.delete(questionSources).where(eq(questionSources.chapterId, stored.id))
        const questions = await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterId, stored.id))
        for (const [index, q] of ch.questions.entries()) {
          await tx.insert(questionSources).values({
            questionId: questions.find(s => s.position === index + 1)!.id,
            chapterId: stored.id,
            matchKey: q.matchKey,
          })
        }
        await tx.delete(chapterAssets).where(eq(chapterAssets.chapterId, stored.id))
        const sourceText = [ch.bodyMarkdown, ...ch.questions.map(q => q.promptMarkdown)].join('\n')
        const refs = new Set(
          new MarkdownIt({ html: false })
            .parse(sourceText, {})
            .flatMap(t => (t.children || []).filter(c => c.type === 'image').map(c => String(c.attrGet('src')))),
        )
        for (const assetId of new Set([...refs].map(h => assetMap.get(h)).filter((id): id is string => !!id))) {
          await tx.insert(chapterAssets).values({ chapterId: stored.id, assetId })
        }
      }

      await tx
        .insert(moduleSources)
        .values({
          moduleId,
          origin,
          sourcePath: mdEntry.path,
          sourceHash: JSON.stringify(newManifest),
          commit,
        })
        .onConflictDoUpdate({
          target: moduleSources.moduleId,
          set: {
            origin,
            sourcePath: mdEntry.path,
            sourceHash: JSON.stringify(newManifest),
            commit,
            syncedAt: new Date(),
          },
        })

      await pruneUnusedCategories(tx)
      return moduleId
    })

    synced.push({
      moduleId: savedId,
      title: parsed.title,
      sourcePath: mdEntry.path,
      action: existingMod ? 'updated' : 'created',
      downloadedImages,
      reusedImages,
    })
  }

  return { synced, commit }
}
