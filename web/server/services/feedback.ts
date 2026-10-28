import { createError } from 'h3'
import { client } from '../db'
import { feedbackInput, feedbackStatus, feedbackId } from '../utils/feedback'
import type { FeedbackList, FeedbackStatus } from '../../shared/feedback'

export async function submitFeedback(user: string, body: unknown) {
  const input = feedbackInput(body)
  return client.begin(async tx => {
    // Serialize only this user's short submissions, so concurrent requests cannot bypass the allowance.
    const [account] = await tx`select status from users where id = ${user} for update`
    if (account?.status !== 'active') throw createError({ statusCode: 401, message: '请登录后提交反馈' })
    const [recent] = await tx`select count(*)::int as count from feedback
      where user_id = ${user} and created_at > now() - interval '10 minutes'`
    if (recent!.count >= 5) throw createError({ statusCode: 429, message: '反馈提交较多，请稍后再试' })
    const [row] = await tx`insert into feedback(user_id, category, body, source_path)
      values(${user}, ${input.category}, ${input.body}, ${input.sourcePath})
      returning id, created_at as "createdAt"`
    return row!
  })
}

export async function listFeedback(page: unknown, status: unknown): Promise<FeedbackList> {
  const number = Number(page ?? 1)
  if (!Number.isSafeInteger(number) || number < 1 || number > 100000) throw createError({ statusCode: 400, message: '页码无效' })
  const filter = status === undefined || status === 'all' ? null : feedbackStatus(status)
  return client.begin('isolation level repeatable read read only', async tx => {
    const counts: Record<FeedbackStatus, number> = { new: 0, in_progress: 0, resolved: 0 }
    for (const row of await tx`select status, count(*)::int as count from feedback group by status`) {
      counts[row.status as FeedbackStatus] = row.count
    }
    const total = filter ? counts[filter] : counts.new + counts.in_progress + counts.resolved
    const where = filter ? tx`f.status = ${filter}` : tx`true`
    const rows = await tx`select f.id, f.category, f.body, f.source_path as "sourcePath", f.status,
      f.created_at as "createdAt", f.updated_at as "updatedAt",
      u.id as "authorId", u.display_name as "authorName", u.email as "authorEmail"
      from feedback f left join users u on u.id = f.user_id where ${where}
      order by f.created_at desc, f.id limit 20 offset ${(number - 1) * 20}`
    return {
      items: rows.map(({ authorId, authorName, authorEmail, ...row }) => ({
        ...row, author: { id: authorId, name: authorName || '已注销用户', email: authorEmail },
      })) as FeedbackList['items'],
      total, page: number, pageSize: 20, counts,
    }
  })
}

export async function updateFeedbackStatus(id: unknown, status: unknown) {
  const value = feedbackId(id), next = feedbackStatus(status)
  const [row] = await client`update feedback set status = ${next},
    updated_at = case when status = ${next} then updated_at else now() end
    where id = ${value} returning id, status, updated_at as "updatedAt"`
  if (!row) throw createError({ statusCode: 404, message: '反馈不存在' })
  return row
}
