import { defineEventHandler, readBody, createError } from 'h3'
import { requireLearner } from '../../utils/learner'
import { client } from '../../db'
export default defineEventHandler(async (e) => {
  const user = requireLearner(e),
    body = await readBody(e),
    name = typeof body?.displayName === 'string' ? body.displayName.trim() : ''
  if (!name || name.length > 64)
    throw createError({ statusCode: 400, message: '昵称需为 1–64 字' })
  await client`update users set display_name=${name},updated_at=now() where id=${user}`
  return { saved: true }
})
