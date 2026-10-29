import path from 'node:path'
import { mkdir, writeFile, readFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
export function contentDataDirectory() {
  return path.resolve(process.env.CONTENT_DATA_DIR || '/var/lib/attackbox-web/content')
}
export function hash(value: string | Buffer) {
  return createHash('sha256').update(value).digest('hex')
}
export async function storeImage(bytes: Buffer) {
  const digest = hash(bytes)
  const directory = path.join(contentDataDirectory(), 'assets')
  await mkdir(directory, { recursive: true, mode: 0o700 })
  const file = path.join(directory, digest)
  try {
    await writeFile(file, bytes, { mode: 0o600, flag: 'wx' })
  } catch (e: any) {
    if (e.code !== 'EEXIST' || hash(await readFile(file)) !== digest) throw e
  }
  return digest
}
