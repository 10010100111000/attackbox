import { defineEventHandler, getHeader, createError } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { uploadLabBundle } from '../../../services/compose-templates'
import { MAX_PACKAGE } from '../../../labs/archive'
export default defineEventHandler(async e => {
  const admin = requireAdmin(e)
  const chunks: Buffer[] = []
  let size = 0
  for await (const chunk of e.node.req) {
    const b = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)
    size += b.length
    if (size > MAX_PACKAGE) throw createError({ statusCode: 413, message: '部署包不能超过 8 MiB' })
    chunks.push(b)
  }
  let filename: string
  try {
    filename = decodeURIComponent(getHeader(e, 'x-lab-filename') || '')
  } catch {
    throw createError({ statusCode: 400, message: '文件名无效' })
  }
  return { data: await uploadLabBundle(admin.id, Buffer.concat(chunks), filename) }
})
