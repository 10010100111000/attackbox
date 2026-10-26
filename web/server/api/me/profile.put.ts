import { defineEventHandler, readBody, createError } from 'h3'
import { requireLearner } from '../../utils/learner'
import { client } from '../../db'
export default defineEventHandler(async (e) => {
  const user = requireLearner(e, '请登录后修改账户资料'),
    body = await readBody(e),
    name = typeof body?.displayName === 'string' ? body.displayName.trim() : ''
  if (!name || name.length > 64)
    throw createError({ statusCode: 400, message: '昵称需为 1–64 字' })
  const [profile] =
    await client`update users set display_name=${name},updated_at=now() where id=${user} returning display_name as "displayName"`
  if (!profile) throw createError({ statusCode: 404, message: '账户不存在' })
  return { saved: true, displayName: profile.displayName as string }
})
