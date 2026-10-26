import { createError } from 'h3'
import { client } from '../db'
export function notificationInput(body: any) {
  const title = typeof body?.title === 'string' ? body.title.trim() : '',
    text = typeof body?.body === 'string' ? body.body.trim() : '',
    link = typeof body?.link === 'string' ? body.link.trim() : ''
  if (!title || title.length > 120 || !text || text.length > 8000)
    throw createError({
      statusCode: 400,
      message: '标题需为 1–120 字，正文需为 1–8000 字',
    })
  if (
    link &&
    (link.length > 500 ||
      !link.startsWith('/') ||
      link.startsWith('//') ||
      /[\\\s\u0000-\u001f]/.test(link))
  )
    throw createError({
      statusCode: 400,
      message: '链接仅支持本站路径，例如 /learn',
    })
  return { title, body: text, link: link || null }
}
export function notificationId(value: unknown) {
  if (
    typeof value !== 'string' ||
    !/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value)
  )
    throw createError({ statusCode: 400, message: '通知标识无效' })
  return value
}
export async function unreadNotifications(user: string) {
  const [r] =
    await client`select count(*)::int as count from announcements a where a.withdrawn_at is null and not exists(select 1 from notification_reads r where r.announcement_id=a.id and r.user_id=${user})`
  return r!.count as number
}
export async function listNotifications(
  user: string,
  page: unknown,
  admin = false,
) {
  const n = Number(page || 1)
  if (!Number.isSafeInteger(n) || n < 1 || n > 100000)
    throw createError({ statusCode: 400, message: '页码无效' })
  return client.begin(async (tx) => {
    const [count] =
      await tx`select count(*)::int as total from announcements where (${admin} or withdrawn_at is null)`
    const items =
      await tx`select a.id,a.title,a.body,a.link,a.published_at as "publishedAt",a.withdrawn_at as "withdrawnAt",r.read_at as "readAt" from announcements a left join notification_reads r on r.announcement_id=a.id and r.user_id=${user} where (${admin} or a.withdrawn_at is null) order by a.published_at desc,a.id limit 20 offset ${(n - 1) * 20}`
    const [unread] =
      await tx`select count(*)::int as count from announcements a where a.withdrawn_at is null and not exists(select 1 from notification_reads r where r.announcement_id=a.id and r.user_id=${user})`
    return {
      items,
      total: count!.total,
      page: n,
      pageSize: 20,
      unread: unread!.count,
    }
  })
}
export async function publishNotification(author: string, body: any) {
  const input = notificationInput(body)
  const [r] =
    await client`insert into announcements(author_id,title,body,link) values(${author},${input.title},${input.body},${input.link}) returning id`
  return r
}
export async function readNotification(user: string, id: unknown) {
  const value = notificationId(id)
  const rows =
    await client`insert into notification_reads(announcement_id,user_id) select id,${user} from announcements where id=${value} and withdrawn_at is null on conflict(announcement_id,user_id) do update set read_at=notification_reads.read_at returning announcement_id`
  if (!rows.length)
    throw createError({ statusCode: 404, message: '通知已撤回或不存在' })
  return { unread: await unreadNotifications(user) }
}
export async function withdrawNotification(id: unknown) {
  const rows =
    await client`update announcements set withdrawn_at=coalesce(withdrawn_at,now()) where id=${notificationId(id)} returning id`
  if (!rows.length)
    throw createError({ statusCode: 404, message: '通知不存在' })
  return { withdrawn: true }
}
