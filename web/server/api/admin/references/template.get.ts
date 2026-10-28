import { defineEventHandler, setResponseHeaders } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { referenceTemplate } from '../../../content/reference-template'
export default defineEventHandler(event => {
  requireAdmin(event)
  setResponseHeaders(event, {'Content-Type':'text/markdown; charset=utf-8','Content-Disposition':`attachment; filename*=UTF-8''${encodeURIComponent('参考资料示例.md')}`})
  return referenceTemplate
})
