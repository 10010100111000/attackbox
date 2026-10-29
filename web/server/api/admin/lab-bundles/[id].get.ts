import { defineEventHandler, getRouterParam, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { previewLabBundle } from '../../../services/compose-templates'
export default defineEventHandler(async e => {
  requireAdmin(e)
  const q = getQuery(e)
  return {
    data: await previewLabBundle(
      getRouterParam(e, 'id')!,
      typeof q.file === 'string' ? q.file : undefined
    )
  }
})
