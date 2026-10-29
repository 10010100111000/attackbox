import path from 'node:path'
import { ContentFormatError, type ContentImage } from './parse-module'
import type { GithubTreeEntry } from './github'
const allowedImages = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif', '.avif'])

export function resolveTreeImageEntry(
  mdPath: string,
  image: ContentImage,
  blobMap: Map<string, GithubTreeEntry>
): GithubTreeEntry {
  let href: string
  try {
    href = decodeURIComponent(image.href)
  } catch {
    throw new ContentFormatError([
      { file: image.file, line: image.line, field: 'image', message: '图片地址编码无效' }
    ])
  }
  if (
    !href ||
    href.includes('\0') ||
    href.includes('\\') ||
    /[:?#]/.test(href) ||
    path.posix.isAbsolute(href)
  ) {
    throw new ContentFormatError([
      { file: image.file, line: image.line, field: 'image', message: '图片必须使用仓库内相对路径' }
    ])
  }
  const baseDir = path.posix.dirname(mdPath)
  const resolved = path.posix.normalize(baseDir === '.' ? href : `${baseDir}/${href}`)
  if (resolved === '..' || resolved.startsWith('../') || path.posix.isAbsolute(resolved)) {
    throw new ContentFormatError([
      { file: image.file, line: image.line, field: 'image', message: '图片路径越过仓库根目录' }
    ])
  }
  const ext = path.posix.extname(resolved).toLowerCase()
  if (!allowedImages.has(ext)) {
    throw new ContentFormatError([
      {
        file: image.file,
        line: image.line,
        field: 'image',
        message: '仅支持 PNG、JPEG、WebP、GIF 或 AVIF 图片'
      }
    ])
  }
  const entry = blobMap.get(resolved)
  if (
    !entry ||
    entry.type !== 'blob' ||
    !entry.size ||
    entry.size <= 0 ||
    entry.size > 10 * 1024 * 1024
  ) {
    throw new ContentFormatError([
      {
        file: image.file,
        line: image.line,
        field: 'image',
        message: `仓库中找不到图片或文件大小无效 (${resolved})`
      }
    ])
  }
  return entry
}
