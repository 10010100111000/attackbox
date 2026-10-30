import { createError, getHeader, type H3Event } from 'h3'
export async function analyticsBody(event: H3Event): Promise<unknown> {
  if (getHeader(event, 'content-type')?.split(';')[0]?.trim() !== 'application/json')
    throw createError({ statusCode: 415, message: '请使用 JSON 提交统计' })
  if (Number(getHeader(event, 'content-length') || 0) > 4096)
    throw createError({ statusCode: 413, message: '统计请求过大' })
  const chunks: Buffer[] = []
  let bytes = 0
  for await (const chunk of event.node.req) {
    const value = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)
    bytes += value.length
    if (bytes > 4096) throw createError({ statusCode: 413, message: '统计请求过大' })
    chunks.push(value)
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    throw createError({ statusCode: 400, message: '统计 JSON 无效' })
  }
}
