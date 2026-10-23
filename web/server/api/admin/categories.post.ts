import { defineEventHandler, readBody } from 'h3'
import { db } from '../../db'
import { contentCategories } from '../../db/schema'
import { requireAdmin } from '../../utils/admin'
import { fail, slugFor } from '../../services/content-input'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const body = await readBody(event)
  const name = typeof body?.name === 'string' ? body.name.trim() : ''
  if (!name || name.length > 80) fail('分类名称不能为空，且不能超过 80 字')
  try {
    const [category] = await db.insert(contentCategories).values({ name, slug: slugFor(body.slug, 'category') }).returning()
    return { ok: true, data: category }
  } catch (error: any) {
    if (error?.code === '23505' || error?.cause?.code === '23505') fail('分类名称或访问标识已存在', 409)
    throw error
  }
})
