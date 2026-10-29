import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { contentRepositorySettings } from '../../../services/content-repository'
export default defineEventHandler(async e => {
  const admin = requireAdmin(e)
  return { data: await contentRepositorySettings(admin.id) }
})
