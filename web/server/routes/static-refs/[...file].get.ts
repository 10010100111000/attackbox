import { defineEventHandler, getRouterParam, setHeader } from 'h3'
import { readReferenceFile } from '../../content/reference-files'
/** Development/preview fallback. Production nginx serves these paths directly. */
export default defineEventHandler(async event => {
  const file = getRouterParam(event, 'file') || ''
  const content = await readReferenceFile(file)
  setHeader(
    event,
    'Content-Type',
    file.endsWith('.json') ? 'application/json; charset=utf-8' : 'text/html; charset=utf-8'
  )
  setHeader(event, 'X-Content-Type-Options', 'nosniff')
  return content
})
