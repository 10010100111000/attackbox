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
  readdir
} from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import { randomUUID } from 'node:crypto'
import { createError } from 'h3'
import { contentDataDirectory, hash } from './storage.ts'
import { compileReference } from './reference-compiler.ts'
import type { ReferenceMetadata, ReferenceReading } from '../../shared/references'

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
export async function buildReference(input: {
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
    JSON.stringify({ title: input.title, summary: input.summary, outline: compiled.outline, html })
  )
  const directory = path.join(privateDirectory(input.id), revision)
  const meta: ReferenceMetadata = {
    id: input.id,
    slug: input.slug,
    title: input.title,
    summary: input.summary,
    revision,
    outline: compiled.outline,
    chunks: compiled.chunks.map((chunk, index) => ({
      index,
      title: chunk.title,
      url: `/static-refs/${input.slug}/${revision}/chunk-${index}.html`
    }))
  }
  await mkdir(directory, { recursive: true, mode: 0o700 })
  for (const [index, body] of html.entries()) {
    await writeFile(path.join(directory, `chunk-${index}.html`), body, { mode: 0o600 })
    await writeFile(path.join(directory, `chunk-${index}.html.gz`), gzipSync(body), { mode: 0o600 })
  }
  // Metadata is written last; incomplete builds are never publishable.
  await writeFile(path.join(directory, 'meta.json'), JSON.stringify(meta), { mode: 0o600 })
  return meta
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
/** DB row locks serialize publication. Pointer swaps are atomic; rollback restores
 * the prior public release if the database transaction fails. */
export async function referencePublication<T>(
  work: (files: {
    publish: (meta: ReferenceMetadata) => Promise<void>
    withdraw: (slug: string) => Promise<void>
  }) => Promise<T>
): Promise<T> {
  const changes: FileChange[] = []
  const replace = async (slug: string, target: string | null) => {
    const previous = await currentPointer(slug)
    if (previous === target) return
    await point(slug, target)
    changes.push({
      rollback: async () => {
        await point(slug, previous)
        if (target)
          await rm(path.join(referencePublicDirectory(), target), { recursive: true, force: true })
      },
      finish: async () => {
        if (previous)
          await rm(path.join(referencePublicDirectory(), previous), {
            recursive: true,
            force: true
          })
      }
    })
  }
  let result: T
  try {
    result = await work({
      async publish(meta) {
        const root = referencePublicDirectory()
        await mkdir(path.join(root, '.releases'), { recursive: true, mode: 0o755 })
        await chmod(root, 0o755)
        await chmod(path.join(root, '.releases'), 0o755)
        const target = `.releases/${meta.slug}-${meta.revision}`
        if ((await currentPointer(meta.slug)) === target) return
        // Stage a complete release outside every public URL, then switch one symlink.
        const directory = path.join(root, target)
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
        await replace(meta.slug, target)
      },
      withdraw: slug => replace(slug, null)
    })
  } catch (error) {
    for (const change of changes.reverse()) await change.rollback()
    throw error
  }
  for (const change of changes)
    await change.finish().catch(error => console.warn('Reference release cleanup failed', error))
  return result
}
export async function pruneReferenceBuilds(id: string, keep?: string) {
  const directory = privateDirectory(id)
  if (!keep) {
    await rm(directory, { recursive: true, force: true })
    return
  }
  for (const name of await readdir(directory))
    if (name !== keep) await rm(path.join(directory, name), { recursive: true, force: true })
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

export async function referenceBuildAvailable(id: string, revision?: string) {
  if (!revision) return false
  try {
    const meta = await compiledReference(id, revision)
    return meta.id === id && meta.revision === revision
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
