import { createError, defineEventHandler, getRouterParam, setHeader } from 'h3'
import { requireAdmin } from '../../../../../../utils/admin'
import { adminReferencePreview } from '../../../../../../services/references'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const chunk = getRouterParam(event, 'chunk')?.match(/^chunk-(\d+)$/)
  if (!chunk) throw createError({ statusCode: 404, message: '预览内容不存在' })
  const result = await adminReferencePreview(
    getRouterParam(event, 'id') || '',
    getRouterParam(event, 'revision') || '',
    Number(chunk[1])
  )
  setHeader(event, 'Content-Type', 'text/html; charset=utf-8')
  return result.html
})
