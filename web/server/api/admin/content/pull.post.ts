import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { pullContentRepository } from '../../../services/content-repository'
export default defineEventHandler(async e => {
  const admin = requireAdmin(e)
  return { data: await pullContentRepository(admin.id) }
})
