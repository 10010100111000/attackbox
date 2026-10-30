import {
  buildReference,
  compiledReference,
  referencePublication,
  pruneReferenceBuilds,
  referenceBuildAvailable,
  publicReferenceSlugs
} from '../content/reference-files'
import path from 'node:path'
import { randomUUID } from 'node:crypto'
import { and, eq, sql, desc } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import {
  referenceDocuments as documents,
  referenceImages,
  assets,
  contentRepository
} from '../db/schema'
import { contentRepositoryCredentials } from './content-repository'
import { fetchGithubTree, fetchGithubBlob } from '../content/github'
import type { GithubTreeEntry } from '../content/github'
import { parseReferenceMarkdown } from '../content/parse-reference'
import { resolveTreeImageEntry } from '../content/github-images'
import { storeImage } from '../content/storage'
import type { ReferenceScan } from '../../shared/references'
import type { ReferenceManifest } from '../db/schema/references'
const media: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.gif': 'image/gif',
  '.avif': 'image/avif'
}
const fail = (code: number, message: string): never => {
  throw createError({ statusCode: code, message })
}
export function referenceId(value: unknown) {
  if (typeof value !== 'string' || !/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value))
    fail(400, '资料标识无效')
  return value as string
}
function version(value: unknown, min = 1) {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < min)
    fail(400, '资料版本无效，请重新读取')
  return value as number
}
const origin = (repository: string, directory: string) =>
  `github:${repository.toLowerCase()}:${directory}`
function discover(entries: GithubTreeEntry[]) {
  const files = entries.filter(
    e =>
      e.type === 'blob' &&
      e.path.endsWith('.md') &&
      !e.path.split('/').some(p => p.startsWith('.')) &&
      path.posix.basename(e.path).toLowerCase() !== 'readme.md'
  )
  if (files.length > 256) fail(400, '单个资料目录最多 256 篇，请缩小扫描目录')
  return files.sort((a, b) => a.path.localeCompare(b.path))
}
function changed(manifest: ReferenceManifest, sha: string, tree: Map<string, GithubTreeEntry>) {
  return (
    !manifest.reading ||
    manifest.md !== sha ||
    Object.values(manifest.images).some(image => tree.get(image.path)?.sha !== image.sha)
  )
}
export async function scanReferences(): Promise<ReferenceScan> {
  const { config, token } = await contentRepositoryCredentials(2, false)
  const { commit, entries } = await fetchGithubTree(config, token)
  const tree = new Map(entries.map(e => [e.path, e]))
  const old = await db
    .select()
    .from(documents)
    .where(eq(documents.sourceOrigin, origin(config.repository, config.directory)))
  const ready = new Set(
    (
      await Promise.all(
        old.map(async doc =>
          (await referenceBuildAvailable(doc.id, doc.manifest.reading)) ? doc.id : null
        )
      )
    ).filter(Boolean)
  )
  const files = discover(entries),
    paths = new Set(files.map(f => f.path))
  return {
    commit,
    version: config.version,
    items: [
      ...files.map(file => {
        const doc = old.find(d => d.sourcePath === file.path)
        return {
          sourcePath: file.path,
          title: path.posix.basename(file.path, '.md'),
          id: doc?.id || null,
          lockVersion: doc?.lockVersion || 0,
          state: !doc
            ? ('new' as const)
            : changed(doc.manifest, file.sha, tree) || !ready.has(doc.id)
              ? ('changed' as const)
              : ('unchanged' as const)
        }
      }),
      ...old
        .filter(doc => !paths.has(doc.sourcePath))
        .map(doc => ({
          sourcePath: doc.sourcePath,
          title: doc.title,
          id: doc.id,
          lockVersion: doc.lockVersion,
          state: 'missing' as const
        }))
    ]
  }
}
export async function importReference(actor: string, body: Record<string, unknown>) {
  if (
    typeof body.sourcePath !== 'string' ||
    typeof body.commit !== 'string' ||
    !/^[a-f0-9]{40}$/.test(body.commit)
  )
    fail(400, '请先扫描并选择资料')
  const expectedVersion = version(body.lockVersion, 0)
  const { config, token } = await contentRepositoryCredentials(2, false)
  if (body.version !== config.version) fail(409, '仓库配置已变化，请重新扫描')
  const { commit, entries } = await fetchGithubTree(config, token)
  if (commit !== body.commit) fail(409, '仓库已有新提交，请重新扫描后导入')
  const file = discover(entries).find(e => e.path === body.sourcePath)
  if (!file || !Number.isSafeInteger(file.size) || file.size! > 2 * 1024 * 1024 || file.size! <= 0)
    fail(400, '资料不存在、为空或超过 2 MiB')
  const parsed = parseReferenceMarkdown(
    file!.path,
    (await fetchGithubBlob(config, token, file!.sha, file!.size)).toString('utf8')
  )
  const tree = new Map(entries.filter(e => e.type === 'blob').map(e => [e.path, e]))
  const images = new Map(
    [...new Map(parsed.images.map(image => [image.href, image])).values()].map(image => [
      image.href,
      resolveTreeImageEntry(file!.path, image, tree)
    ])
  )
  if (
    [...new Map([...images.values()].map(e => [e.sha, e])).values()].reduce(
      (sum, e) => sum + (e.size || 0),
      0
    ) >
    64 * 1024 * 1024
  )
    fail(400, '单篇资料图片总大小不能超过 64 MiB')
  const sourceOrigin = origin(config.repository, config.directory)
  const manifest: ReferenceManifest = {
    md: file!.sha,
    images: Object.fromEntries([...images].map(([href, e]) => [href, { path: e.path, sha: e.sha }]))
  }
  const blobs = new Map<
    string,
    { digest: string; bytes: number; mediaType: string; name: string }
  >()
  for (const image of images.values()) {
    if (blobs.has(image.sha)) continue
    const bytes = await fetchGithubBlob(config, token, image.sha, image.size)
    blobs.set(image.sha, {
      digest: await storeImage(bytes),
      bytes: bytes.length,
      mediaType: media[path.posix.extname(image.path).toLowerCase()]!,
      name: path.posix.basename(image.path).slice(0, 255)
    })
  }
  const imported = await referencePublication(files =>
    db.transaction(async tx => {
      const [settings] = await tx
        .select()
        .from(contentRepository)
        .where(eq(contentRepository.id, 2))
        .for('share')
      if (settings?.version !== config.version) fail(409, '仓库配置已变化，请重新扫描')
      await tx.execute(
        sql`select pg_advisory_xact_lock(hashtextextended(${JSON.stringify([sourceOrigin, file!.path])},0))`
      )
      const [old] = await tx
        .select()
        .from(documents)
        .where(and(eq(documents.sourceOrigin, sourceOrigin), eq(documents.sourcePath, file!.path)))
        .for('update')
      if ((old?.lockVersion || 0) !== expectedVersion)
        fail(409, '资料已被其他管理员修改，请重新扫描')
      if (
        old &&
        !changed(old.manifest, file!.sha, tree) &&
        (await referenceBuildAvailable(old.id, old.manifest.reading))
      )
        return { id: old.id, unchanged: true }
      const id = old?.id || randomUUID()
      const values = {
        title: parsed.title,
        summary: parsed.summary,
        bodyMarkdown: parsed.bodyMarkdown,
        status: 'unpublished' as const,
        sourceCommit: commit,
        manifest,
        updatedAt: new Date()
      }
      if (old)
        await tx
          .update(documents)
          .set({ ...values, lockVersion: old.lockVersion + 1 })
          .where(eq(documents.id, id))
      else
        await tx.insert(documents).values({
          ...values,
          id,
          slug: `reference-${id}`,
          sourceOrigin,
          sourcePath: file!.path,
          createdBy: actor
        })
      const imageUrls: Record<string, string> = {}
      const slug = old?.slug || `reference-${id}`
      await tx.delete(referenceImages).where(eq(referenceImages.referenceId, id))
      for (const [href, image] of images) {
        const blob = blobs.get(image.sha)!
        await tx
          .insert(assets)
          .values({
            storageKey: blob.digest,
            originalName: blob.name,
            mediaType: blob.mediaType,
            byteSize: blob.bytes,
            sha256: Buffer.from(blob.digest, 'hex'),
            visibility: 'private',
            uploadedBy: actor
          })
          .onConflictDoNothing({ target: assets.storageKey })
        const [asset] = await tx
          .select({ id: assets.id })
          .from(assets)
          .where(eq(assets.storageKey, blob.digest))
        await tx.insert(referenceImages).values({ referenceId: id, href, assetId: asset!.id })
        imageUrls[href] = `/api/references/${slug}/assets/${asset!.id}`
      }
      const built = await buildReference({
        id,
        slug,
        title: parsed.title,
        summary: parsed.summary,
        bodyMarkdown: parsed.bodyMarkdown,
        imageUrls
      })
      await tx
        .update(documents)
        .set({ manifest: { ...manifest, reading: built.revision } })
        .where(eq(documents.id, id))
      if (old) await files.withdraw(old.slug)
      return { id, unchanged: false, revision: built.revision }
    })
  )
  await cleanupReferenceBuilds(imported.id)
  return { id: imported.id, unchanged: imported.unchanged }
}
const summaryFields = {
  id: documents.id,
  slug: documents.slug,
  title: documents.title,
  summary: documents.summary,
  updatedAt: documents.updatedAt,
  publishedAt: documents.publishedAt
}
export async function listReferences(rawPage: unknown, admin = false) {
  const page = rawPage === undefined ? 1 : Number(rawPage)
  if (!Number.isSafeInteger(page) || page < 1 || page > 100000) fail(400, '页码无效')
  return db.transaction(
    async tx => {
      const condition = admin ? undefined : eq(documents.status, 'published')
      const [count] = await tx
        .select({ total: sql<number>`count(*)::int` })
        .from(documents)
        .where(condition)
      const items = await tx
        .select({
          ...summaryFields,
          ...(admin
            ? {
                status: documents.status,
                lockVersion: documents.lockVersion,
                sourcePath: documents.sourcePath
              }
            : {})
        })
        .from(documents)
        .where(condition)
        .orderBy(desc(documents.updatedAt), desc(documents.id))
        .limit(24)
        .offset((page - 1) * 24)
      return { items, total: count!.total, page, pageSize: 24 }
    },
    { isolationLevel: 'repeatable read', accessMode: 'read only' }
  )
}
export async function adminReferenceDetail(key: string) {
  referenceId(key)
  return db.transaction(
    async tx => {
      const [doc] = await tx
        .select({
          ...summaryFields,
          bodyMarkdown: documents.bodyMarkdown,
          status: documents.status,
          lockVersion: documents.lockVersion,
          sourcePath: documents.sourcePath,
          sourceCommit: documents.sourceCommit
        })
        .from(documents)
        .where(eq(documents.id, key))
      if (!doc) fail(404, '资料不存在或尚未发布')
      const images = await tx
        .select()
        .from(referenceImages)
        .where(eq(referenceImages.referenceId, doc!.id))
      const prefix = `/api/admin/references/${doc!.id}`
      return {
        ...doc!,
        imageUrls: Object.fromEntries(
          images.map(image => [image.href, `${prefix}/assets/${image.assetId}`])
        )
      }
    },
    { isolationLevel: 'repeatable read', accessMode: 'read only' }
  )
}
async function cleanupReferenceBuilds(id: string) {
  // Re-read under the same per-document lock: a newer import may have committed
  // before this cleanup runs, or may currently be writing its own build.
  await db
    .transaction(async tx => {
      const [current] = await tx
        .select({ manifest: documents.manifest })
        .from(documents)
        .where(eq(documents.id, id))
        .for('update')
      await pruneReferenceBuilds(id, current?.manifest.reading)
    })
    .catch(error => console.warn('Reference build cleanup failed', error))
}
export async function changeReference(id: string, body: Record<string, unknown>, remove = false) {
  referenceId(id)
  const expected = version(body.lockVersion)
  if (!remove && body.intent !== 'publish' && body.intent !== 'unpublish') fail(400, '发布操作无效')
  const result = await referencePublication(files =>
    db.transaction(async tx => {
      const [doc] = await tx.select().from(documents).where(eq(documents.id, id)).for('update')
      if (!doc) fail(404, '资料不存在')
      if (doc!.lockVersion !== expected) fail(409, '资料已变化，请重新读取后操作')
      if (!remove && body.intent === 'publish') {
        if (!doc!.manifest.reading) fail(409, '请先重新导入资料，生成静态正文')
        await files.publish(await compiledReference(doc!.id, doc!.manifest.reading!))
      } else await files.withdraw(doc!.slug)
      if (remove) await tx.delete(documents).where(eq(documents.id, id))
      else
        await tx
          .update(documents)
          .set({
            status: body.intent === 'publish' ? 'published' : 'unpublished',
            publishedAt: body.intent === 'publish' ? new Date() : doc!.publishedAt,
            lockVersion: expected + 1,
            updatedAt: new Date()
          })
          .where(eq(documents.id, id))
      return { saved: true }
    })
  )
  if (remove) await cleanupReferenceBuilds(id)
  return result
}
export async function referenceAsset(key: string, assetId: string, admin = false) {
  referenceId(assetId)
  if (admin) referenceId(key)
  const [row] = await db
    .select({ asset: assets })
    .from(referenceImages)
    .innerJoin(documents, eq(referenceImages.referenceId, documents.id))
    .innerJoin(assets, eq(referenceImages.assetId, assets.id))
    .where(
      and(
        eq(referenceImages.assetId, assetId),
        admin
          ? eq(documents.id, key)
          : and(eq(documents.slug, key), eq(documents.status, 'published'))
      )
    )
  if (!row || !/^image\/(png|jpeg|webp|gif|avif)$/.test(row.asset.mediaType))
    fail(404, '图片不存在')
  return row!.asset
}

/** One startup reconciliation repairs a process crash between filesystem and DB
 * commits. It never compiles content and adds no request-time database reads. */
export async function reconcileReferencePublications() {
  const rows = await db
    .select({
      id: documents.id,
      slug: documents.slug,
      status: documents.status,
      manifest: documents.manifest
    })
    .from(documents)
  const slugs = new Set(rows.map(row => row.slug))
  for (const row of rows) {
    await referencePublication(async files => {
      if (
        row.status === 'published' &&
        (await referenceBuildAvailable(row.id, row.manifest.reading))
      )
        await files.publish(await compiledReference(row.id, row.manifest.reading!))
      else await files.withdraw(row.slug)
    })
  }
  for (const slug of await publicReferenceSlugs())
    if (!slugs.has(slug)) await referencePublication(async files => files.withdraw(slug))
}
