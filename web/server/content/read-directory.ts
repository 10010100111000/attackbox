import path from 'node:path'
import { lstat, readdir, readFile, realpath } from 'node:fs/promises'
import { ContentFormatError, parseModuleMarkdown } from './parse-module.ts'
import type { ParsedModule, ContentImage, ContentIssue } from './parse-module.ts'

export interface ResolvedContentImage extends ContentImage { repositoryPath: string; size: number }
export interface CheckedModule { module: ParsedModule; images: ResolvedContentImage[] }
const allowedImages = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif', '.avif'])
const inside = (root: string, candidate: string) => { const relative = path.relative(root, candidate); return relative !== '..' && !relative.startsWith('..' + path.sep) && !path.isAbsolute(relative) }
function fail(image: ContentImage, message: string): never { throw new ContentFormatError([{ file: image.file, line: image.line, field: 'image', message }]) }

export async function resolveContentImage(rootDirectory: string, image: ContentImage): Promise<ResolvedContentImage> {
  const root = await realpath(rootDirectory)
  let href: string
  try { href = decodeURIComponent(image.href) } catch { return fail(image, '图片地址编码无效') }
  if (!href || href.includes('\0') || href.includes('\\') || /[:?#]/.test(href) || path.isAbsolute(href)) fail(image, '图片必须使用仓库内相对路径，不支持外链、Data URL 或查询参数')
  const candidate = path.resolve(root, path.dirname(image.file), href)
  if (!inside(root, candidate)) fail(image, '图片路径越过仓库根目录')
  if (!allowedImages.has(path.extname(candidate).toLowerCase())) fail(image, '仅支持 PNG、JPEG、WebP、GIF 或 AVIF 图片')
  try {
    // Reject symlinks in every component, including ones whose final target is inside root.
    let cursor = root
    for (const component of path.relative(root, candidate).split(path.sep)) {
      cursor = path.join(cursor, component)
      if ((await lstat(cursor)).isSymbolicLink()) fail(image, '图片路径不能包含符号链接')
    }
    const resolved = await realpath(candidate)
    if (!inside(root, resolved)) fail(image, '图片路径越过仓库根目录')
    const stat = await lstat(resolved)
    if (!stat.isFile() || stat.size === 0 || stat.size > 10 * 1024 * 1024) fail(image, '图片必须是非空文件，且不能超过 10 MiB')
    return { ...image, repositoryPath: path.relative(root, resolved).split(path.sep).join('/'), size: stat.size }
  } catch (error) {
    if (error instanceof ContentFormatError) throw error
    return fail(image, '找不到图片或无法读取文件')
  }
}

/** Local validation only: no DB, network, Git, image copying or publishing. */
export async function checkContentDirectory(directory: string): Promise<CheckedModule[]> {
  const root = await realpath(directory)
  const entries = (await readdir(root, { withFileTypes: true }))
    .filter(e => e.name.endsWith('.md') && !e.name.startsWith('.') && e.name.toLowerCase() !== 'readme.md')
    .sort((a, b) => a.name.localeCompare(b.name))
  if (!entries.length) throw new ContentFormatError([{ file: '.', line: 1, message: '内容目录中没有模块 .md 文件' }])
  const result: CheckedModule[] = [], issues: ContentIssue[] = [], names = new Set<string>()
  let totalBytes = 0
  for (const entry of entries) {
    try {
      const location = { file: entry.name, line: 1 }
      if (!entry.isFile()) throw new ContentFormatError([{ ...location, message: '模块必须是普通文件，不能是目录或符号链接' }])
      const file = path.join(root, entry.name), stat = await lstat(file)
      if (stat.size > 2 * 1024 * 1024) throw new ContentFormatError([{ ...location, message: '单个教程不能超过 2 MiB' }])
      totalBytes += stat.size
      if (totalBytes > 64 * 1024 * 1024) throw new ContentFormatError([{ ...location, message: '单次校验的教程总大小不能超过 64 MiB' }])
      const module = parseModuleMarkdown(entry.name, await readFile(file, 'utf8'))
      const key = module.title.normalize('NFKC')
      if (names.has(key)) throw new ContentFormatError([{ ...location, message: '模块名称重复' }])
      names.add(key)
      const images: ResolvedContentImage[] = []
      for (const image of module.images) images.push(await resolveContentImage(root, image))
      result.push({ module, images })
    } catch (error) {
      if (error instanceof ContentFormatError) issues.push(...error.issues)
      else throw error
    }
  }
  if (issues.length) throw new ContentFormatError(issues)
  return result
}
