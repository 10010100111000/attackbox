import { createError } from 'h3'
import { client } from '../db'
export function notificationInput(body: any) {
  const title = typeof body?.title === 'string' ? body.title.trim() : '',
    text = typeof body?.body === 'string' ? body.body.trim() : '',
    link = typeof body?.link === 'string' ? body.link.trim() : ''
  if (!title || title.length > 120 || !text || text.length > 8000)
    throw createError({
      statusCode: 400,
      message: '标题需为 1–120 字，正文需为 1–8000 字'
    })
  if (
    link &&
    (link.length > 500 ||
      !link.startsWith('/') ||
      link.startsWith('//') ||
      // eslint-disable-next-line no-control-regex -- 显式拒绝控制字符,防止不可见字符注入
      /[\\\s\u0000-\u001f]/.test(link))
  )
    throw createError({
      statusCode: 400,
      message: '链接仅支持本站路径，例如 /learn'
    })
  return { title, body: text, link: link || null }
}
export function notificationId(value: unknown) {
  if (typeof value !== 'string' || !/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value))
    throw createError({ statusCode: 400, message: '通知标识无效' })
  return value
}
export async function unreadNotifications(user: string) {
  const [row] =
    await client`select count(*)::int as count from notifications n where n.withdrawn_at is null and (n.kind='announcement' or n.recipient_id=${user}) and not exists (select 1 from notification_receipts r where r.notification_id=n.id and r.user_id=${user} and (r.read_at is not null or r.dismissed_at is not null))`
  return row!.count as number
}
export async function listNotifications(user: string, page: unknown, admin = false) {
  const n = Number(page || 1)
  if (!Number.isSafeInteger(n) || n < 1 || n > 100000)
    throw createError({ statusCode: 400, message: '页码无效' })
  return client.begin('isolation level repeatable read read only', async tx => {
    const visible = admin
      ? tx`n.kind='announcement'`
      : tx`n.withdrawn_at is null and (n.kind='announcement' or n.recipient_id=${user}) and r.dismissed_at is null`
    const [count] =
      await tx`select count(*)::int as total from notifications n left join notification_receipts r on r.notification_id=n.id and r.user_id=${user} where ${visible}`
    const rows =
      await tx`select n.id,n.kind,n.title,n.body,n.link,n.published_at as "publishedAt",n.withdrawn_at as "withdrawnAt",r.read_at as "readAt",u.id as "senderId",u.display_name as "senderName" from notifications n left join users u on u.id=n.sender_id left join notification_receipts r on r.notification_id=n.id and r.user_id=${user} where ${visible} order by n.published_at desc,n.id limit 20 offset ${(n - 1) * 20}`
    const [unread] =
      await tx`select count(*)::int as count from notifications n where n.withdrawn_at is null and (n.kind='announcement' or n.recipient_id=${user}) and not exists(select 1 from notification_receipts r where r.notification_id=n.id and r.user_id=${user} and (r.read_at is not null or r.dismissed_at is not null))`
    return {
      items: rows.map(({ senderId, senderName, ...row }) => ({
        ...row,
        sender: {
          id: senderId,
          name: senderName || (row.kind === 'announcement' ? 'AttackBox' : '已注销用户')
        }
      })),
      total: count!.total,
      page: n,
      pageSize: 20,
      unread: unread!.count
    }
  })
}
export async function publishNotification(author: string, body: unknown) {
  const input = notificationInput(body)
  const [row] =
    await client`insert into notifications(kind,sender_id,title,body,link) values('announcement',${author},${input.title},${input.body},${input.link}) returning id`
  return row
}
export async function readNotification(user: string, id: unknown) {
  const value = notificationId(id)
  const rows =
    await client`insert into notification_receipts(notification_id,user_id,read_at) select n.id,${user},now() from notifications n where n.id=${value} and n.withdrawn_at is null and (n.kind='announcement' or n.recipient_id=${user}) and not exists(select 1 from notification_receipts r where r.notification_id=n.id and r.user_id=${user} and r.dismissed_at is not null) on conflict(notification_id,user_id) do update set read_at=coalesce(notification_receipts.read_at,excluded.read_at) where notification_receipts.dismissed_at is null returning notification_id`
  if (!rows.length) throw createError({ statusCode: 404, message: '消息已删除、撤回或不存在' })
  return { unread: await unreadNotifications(user) }
}
export async function dismissNotification(user: string, id: unknown) {
  const value = notificationId(id)
  const rows =
    await client`insert into notification_receipts(notification_id,user_id,dismissed_at) select n.id,${user},now() from notifications n where n.id=${value} and n.withdrawn_at is null and (n.kind='announcement' or n.recipient_id=${user}) on conflict(notification_id,user_id) do update set dismissed_at=coalesce(notification_receipts.dismissed_at,excluded.dismissed_at) returning notification_id`
  if (!rows.length) throw createError({ statusCode: 404, message: '消息不存在' })
  return { deleted: true, unread: await unreadNotifications(user) }
}
export async function withdrawNotification(id: unknown) {
  const rows =
    await client`update notifications set withdrawn_at=coalesce(withdrawn_at,now()) where id=${notificationId(id)} and kind='announcement' returning id`
  if (!rows.length) throw createError({ statusCode: 404, message: '通知不存在' })
  return { withdrawn: true }
}
