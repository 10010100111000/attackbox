import path from 'node:path'
import {
  mkdir,
  writeFile,
  readFile,
  chmod,
  cp,
  symlink,
  rename,
  readlink,
  unlink,
  rm,
  readdir,
  mkdtemp
} from 'node:fs/promises'
import { gzip as compress } from 'node:zlib'
import { promisify } from 'node:util'
import { randomUUID } from 'node:crypto'
import { createError } from 'h3'
import { contentDataDirectory, hash } from './storage.ts'
import { compileReference, referenceRendererVersion } from './reference-compiler.ts'
import type { ReferenceMetadata, ReferenceReading } from '../../shared/references'

const gzip = promisify(compress)
export const referencePublicDirectory = () =>
  path.resolve(process.env.REFERENCE_PUBLIC_DIR || '/var/lib/attackbox-public/references')
const privateDirectory = (id: string) => path.join(contentDataDirectory(), 'reference-builds', id)
function validSlug(slug: string) {
  if (!/^[a-zA-Z0-9_-]{1,100}$/.test(slug))
    throw createError({ statusCode: 404, message: '资料不存在或尚未发布' })
  return slug
}
async function missing(error: unknown) {
  if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error
}
export async function prepareReferenceBuild(input: {
  id: string
  slug: string
  title: string
  summary: string
  bodyMarkdown: string
  imageUrls: Record<string, string>
}) {
  validSlug(input.slug)
  const compiled = compileReference(input.bodyMarkdown, input.imageUrls)
  const html = compiled.chunks.map(chunk => chunk.html)
  const revision = hash(
    JSON.stringify({
      rendererVersion: referenceRendererVersion,
      title: input.title,
      summary: input.summary,
      outline: compiled.outline,
      html
    })
  )
  const staging = path.join(contentDataDirectory(), '.reference-staging')
  await mkdir(staging, { recursive: true, mode: 0o700 })
  const directory = await mkdtemp(path.join(staging, 'build-'))
  const meta: ReferenceMetadata = {
    id: input.id,
    slug: input.slug,
    title: input.title,
    summary: input.summary,
    revision,
    rendererVersion: referenceRendererVersion,
    outline: compiled.outline,
    chunks: compiled.chunks.map((chunk, index) => ({
      index,
      title: chunk.title,
      url: `/static-refs/${input.slug}/${revision}/chunk-${index}.html`
    }))
  }
  try {
    for (const [index, body] of html.entries()) {
      await writeFile(path.join(directory, `chunk-${index}.html`), body, { mode: 0o600 })
      await writeFile(path.join(directory, `chunk-${index}.html.gz`), await gzip(body), {
        mode: 0o600
      })
    }
    await writeFile(path.join(directory, 'meta.json'), JSON.stringify(meta), { mode: 0o600 })
  } catch (error) {
    await rm(directory, { recursive: true, force: true })
    throw error
  }
  return {
    meta,
    async install() {
      await mkdir(privateDirectory(input.id), { recursive: true, mode: 0o700 })
      try {
        // In-progress builds live outside the installed tree. Cleanup can never
        // remove a concurrent import's compilation directory.
        await rename(directory, path.join(privateDirectory(input.id), revision))
      } catch (error) {
        if (!['EEXIST', 'ENOTEMPTY'].includes((error as NodeJS.ErrnoException).code || ''))
          throw error
        // An identical immutable build was installed by another request.
      }
    },
    discard: () => rm(directory, { recursive: true, force: true })
  }
}
/** Convenience for fixtures/offline builders; request handlers prepare before locking. */
export async function buildReference(input: Parameters<typeof prepareReferenceBuild>[0]) {
  const prepared = await prepareReferenceBuild(input)
  try {
    await prepared.install()
    return prepared.meta
  } finally {
    await prepared.discard()
  }
}

export async function compiledReference(id: string, revision: string) {
  return JSON.parse(
    await readFile(path.join(privateDirectory(id), revision, 'meta.json'), 'utf8')
  ) as ReferenceMetadata
}
async function currentPointer(slug: string) {
  try {
    return await readlink(path.join(referencePublicDirectory(), validSlug(slug)))
  } catch (error) {
    await missing(error)
    return null
  }
}
async function point(slug: string, target: string | null) {
  const destination = path.join(referencePublicDirectory(), validSlug(slug))
  if (!target) {
    await unlink(destination).catch(missing)
    return
  }
  const temporary = path.join(referencePublicDirectory(), `.pointer-${randomUUID()}`)
  try {
    await symlink(target, temporary)
    await rename(temporary, destination)
  } finally {
    await unlink(temporary).catch(missing)
  }
}
interface FileChange {
  rollback: () => Promise<void>
  finish: () => Promise<void>
}
/** Copy a complete hidden release before acquiring any database lock. */
async function prepareRelease(meta: ReferenceMetadata) {
  const root = referencePublicDirectory()
  await mkdir(path.join(root, '.releases'), { recursive: true, mode: 0o755 })
  await chmod(root, 0o755)
  await chmod(path.join(root, '.releases'), 0o755)
  const target = `.releases/${meta.slug}-${meta.revision}-${randomUUID()}`
  const directory = path.join(root, target)
  try {
    await mkdir(path.join(directory, meta.revision), { recursive: true, mode: 0o755 })
    await chmod(directory, 0o755)
    await chmod(path.join(directory, meta.revision), 0o755)
    const source = path.join(privateDirectory(meta.id), meta.revision)
    for (const file of await readdir(source)) {
      const destination = path.join(
        directory,
        file === 'meta.json' ? file : `${meta.revision}/${file}`
      )
      await cp(path.join(source, file), destination)
      await chmod(destination, 0o644)
    }
    return target
  } catch (error) {
    await rm(directory, { recursive: true, force: true })
    throw error
  }
}
/** Prepare before work() opens its transaction. Only the pointer swap occurs
 * under row locks. Rollback and obsolete release cleanup occur after work(). */
export async function referencePublication<T>(
  work: (files: {
    publish: () => Promise<void>
    withdraw: (slug: string) => Promise<void>
  }) => Promise<T>,
  publication?: ReferenceMetadata
): Promise<T> {
  const target = publication ? await prepareRelease(publication) : undefined
  const changes: FileChange[] = []
  const garbage = new Map<string, string>()
  if (target) garbage.set(target, publication!.slug)
  const replace = async (slug: string, next: string | null) => {
    const previous = await currentPointer(slug)
    if (previous === next) return
    await point(slug, next)
    changes.push({
      rollback: async () => {
        // Do not overwrite a newer successful publication after DB rollback
        // has released the row lock.
        if ((await currentPointer(slug)) === next) await point(slug, previous)
      },
      finish: async () => {
        if (previous) garbage.set(previous, slug)
      }
    })
  }
  try {
    let result: T
    try {
      result = await work({
        async publish() {
          if (!publication || !target) throw new Error('Reference release was not prepared')
          await replace(publication.slug, target)
        },
        withdraw: slug => replace(slug, null)
      })
    } catch (error) {
      for (const change of changes.reverse()) await change.rollback()
      throw error
    }
    for (const change of changes) await change.finish()
    if (target && (await currentPointer(publication!.slug)) === target) garbage.delete(target)
    return result
  } finally {
    for (const [obsolete, slug] of garbage) {
      // A failed rollback must not delete the release still serving readers.
      if ((await currentPointer(slug)) === obsolete) continue
      await rm(path.join(referencePublicDirectory(), obsolete), {
        recursive: true,
        force: true
      }).catch(error => console.warn('Reference release cleanup failed', error))
    }
  }
}
/** Retire obsolete builds with renames while the row is locked; remove their
 * contents after committing, so large trees never hold a database connection. */
export async function retireReferenceBuilds(id: string, keep?: string) {
  const directory = privateDirectory(id)
  let names: string[]
  try {
    names = await readdir(directory)
  } catch (error) {
    await missing(error)
    return async () => {}
  }
  const trash = path.join(contentDataDirectory(), '.reference-trash')
  await mkdir(trash, { recursive: true, mode: 0o700 })
  const retired: string[] = []
  for (const name of names) {
    if (name === keep) continue
    const target = path.join(trash, randomUUID())
    await rename(path.join(directory, name), target)
    retired.push(target)
  }
  return async () => {
    for (const target of retired) await rm(target, { recursive: true, force: true })
  }
}
export async function pruneReferenceBuilds(id: string, keep?: string) {
  await (
    await retireReferenceBuilds(id, keep)
  )()
}
export async function readReferenceFile(relative: string) {
  if (!/^[a-zA-Z0-9_-]{1,100}\/(?:meta\.json|[a-f0-9]{64}\/chunk-\d+\.html)$/.test(relative))
    throw createError({ statusCode: 404, message: '资料文件不存在' })
  try {
    return await readFile(path.join(referencePublicDirectory(), relative), 'utf8')
  } catch (error) {
    await missing(error)
    throw createError({ statusCode: 404, message: '资料不存在或尚未发布' })
  }
}
export async function readReference(slug: string): Promise<ReferenceReading> {
  const meta = JSON.parse(
    await readReferenceFile(`${validSlug(slug)}/meta.json`)
  ) as ReferenceMetadata
  const html = await readReferenceFile(`${slug}/${meta.revision}/chunk-0.html`)
  return { meta, html }
}

export async function referenceBuildAvailable(
  id: string,
  revision?: string,
  options: { currentRenderer?: boolean } = {}
) {
  if (!revision) return false
  try {
    const meta = await compiledReference(id, revision)
    return (
      meta.id === id &&
      meta.revision === revision &&
      (!options.currentRenderer || meta.rendererVersion === referenceRendererVersion)
    )
  } catch (error) {
    if (error instanceof SyntaxError || (error as NodeJS.ErrnoException).code === 'ENOENT')
      return false
    throw error
  }
}
export async function publicReferenceSlugs() {
  try {
    return (await readdir(referencePublicDirectory())).filter(name =>
      /^[a-zA-Z0-9_-]{1,100}$/.test(name)
    )
  } catch (error) {
    await missing(error)
    return []
  }
}

/** Private compiled chunks are served only behind the administrator API. */
export async function readCompiledChunk(id: string, revision: string, index: number) {
  if (!/^[a-f0-9]{64}$/.test(revision) || !Number.isSafeInteger(index) || index < 0)
    throw createError({ statusCode: 404, message: '预览内容不存在' })
  try {
    return await readFile(path.join(privateDirectory(id), revision, `chunk-${index}.html`), 'utf8')
  } catch (error) {
    await missing(error)
    throw createError({ statusCode: 404, message: '预览内容不存在，请重新导入资料' })
  }
}
