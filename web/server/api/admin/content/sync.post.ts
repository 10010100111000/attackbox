import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { syncContentFromRepository } from '../../../services/content-repository'

export default defineEventHandler(async event => {
  const admin = requireAdmin(event)
  const body = (await readBody(event).catch(() => ({}))) || {}
  const sourcePath = typeof body.sourcePath === 'string' && body.sourcePath.trim() ? body.sourcePath.trim() : undefined
  const moduleId = typeof body.moduleId === 'string' && body.moduleId.trim() ? body.moduleId.trim() : undefined
  const force = Boolean(body.force)
  return {
    data: await syncContentFromRepository(admin.id, { sourcePath, moduleId, force }),
  }
})
