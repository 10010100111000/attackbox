import { createHash, randomUUID } from 'node:crypto'
import {
  createError,
  getCookie,
  setCookie,
  getRequestURL,
  setResponseHeader,
  type H3Event
} from 'h3'
import { client } from '../db'
import { clientIp } from '../utils/client-ip'
import {
  parseVisit,
  analyticsId,
  isAnalyticsId,
  visitSource,
  type AnalyticsReport
} from '../../shared/analytics'
import { isAdPlacement } from '../../shared/advertising'
const visitorCookie = 'attackbox_visitor'
const digest = (value: string) => createHash('sha256').update(value).digest('hex')
const validId = (value: unknown) => {
  try {
    return analyticsId(value)
  } catch {
    throw createError({ statusCode: 400, message: '统计标识无效' })
  }
}
function visitor(event: H3Event, create: boolean) {
  const value = getCookie(event, visitorCookie)
  if (isAnalyticsId(value)) return digest(value)
  if (!create) return null
  const id = randomUUID()
  setCookie(event, visitorCookie, id, {
    httpOnly: true,
    sameSite: 'lax',
    secure: getRequestURL(event).protocol === 'https:',
    path: '/',
    maxAge: 86400 * 30
  })
  return digest(id)
}
export async function analyticsBudget(event: H3Event) {
  if (event.context.user?.role === 'admin') return false
  const key = digest(
    `analytics:${new Date().toISOString().slice(0, 10)}:${clientIp(event) || 'unknown'}`
  )
  const [row] =
    await client`insert into analytics_budgets(key,attempts,window_at) values(${key},1,now()) on conflict(key) do update set attempts=case when analytics_budgets.window_at<now()-interval '1 minute' then 1 else least(analytics_budgets.attempts+1,601) end, window_at=case when analytics_budgets.window_at<now()-interval '1 minute' then now() else analytics_budgets.window_at end returning attempts`
  if (row!.attempts > 600) {
    setResponseHeader(event, 'Retry-After', 60)
    throw createError({ statusCode: 429, message: '统计请求较多' })
  }
  return true
}
export async function recordVisit(event: H3Event, body: unknown) {
  let input
  try {
    input = parseVisit(body)
  } catch (error) {
    throw createError({ statusCode: 400, message: (error as Error).message })
  }
  if (!(await analyticsBudget(event))) return { accepted: false }
  const key = visitor(event, true)!
  const source = visitSource(input, getRequestURL(event).hostname)
  const [row] =
    await client`insert into analytics_visits(id,visitor_key,path,source,medium,campaign,channel) values(${input.id},${key},${input.path},${source.source},${source.medium},${source.campaign},${source.channel}) on conflict(id) do nothing returning id`
  if (!row) {
    const [existing] =
      await client`select id from analytics_visits where id=${input.id} and visitor_key=${key} and path=${input.path}`
    if (!existing) throw createError({ statusCode: 409, message: '访问记录冲突' })
  }
  // Retain 90 days, with bounded indexed cleanup and no background timer.
  await client`delete from analytics_visits where id in (select id from analytics_visits where created_at<now()-interval '90 days' limit 100)`
  await client`delete from analytics_budgets where key in (select key from analytics_budgets where window_at<now()-interval '1 day' limit 20)`
  return { accepted: true, id: input.id }
}
export async function recordRead(event: H3Event, body: unknown) {
  const id = validId((body as { visitId?: unknown })?.visitId)
  if (!(await analyticsBudget(event))) return
  const key = visitor(event, false)
  if (!key) throw createError({ statusCode: 403, message: '缺少访问记录' })
  const rows =
    await client`update analytics_visits set read_at=coalesce(read_at,now()) where id=${id} and visitor_key=${key} and created_at>now()-interval '24 hours' and (path like '/references/%' or path like '/learn/modules/%/%') returning id`
  if (!rows.length) throw createError({ statusCode: 409, message: '阅读记录尚不可用' })
}
export async function recordAdEvent(event: H3Event, body: unknown) {
  const input = body as { visitId?: unknown; adId?: unknown; placement?: unknown; kind?: unknown }
  const id = validId(input?.visitId),
    ad = validId(input?.adId)
  if (!isAdPlacement(input.placement) || !['impression', 'click'].includes(String(input.kind)))
    throw createError({ statusCode: 400, message: '广告事件无效' })
  if (!(await analyticsBudget(event))) return
  const key = visitor(event, false)
  if (!key) throw createError({ statusCode: 403, message: '缺少访问记录' })
  // Both facts are idempotent per page visit/creative/placement. A quick click
  // also proves exposure, so legitimate clicks never exceed impressions.
  const kinds = input.kind === 'click' ? ['impression', 'click'] : ['impression']
  const rows = await client`insert into ad_events(visit_id,ad_id,ad_name,placement,kind)
    select v.id,a.id,a.name,${input.placement},k.kind from analytics_visits v cross join advertisements a cross join unnest(${kinds}::text[]) as k(kind)
    where v.id=${id} and v.visitor_key=${key} and v.created_at>now()-interval '24 hours'
    and a.id=${ad} and a.enabled and ${input.placement}=any(a.placements) and (a.starts_at is null or a.starts_at<=now()) and (a.ends_at is null or a.ends_at>now())
    and ((${input.placement}='learn_catalog' and v.path='/learn') or (${input.placement}='reference_footer' and v.path like '/references/%'))
    on conflict do nothing returning visit_id`
  return { recorded: rows.length > 0 }
}
export async function analyticsReport(days: number): Promise<AnalyticsReport> {
  if (![7, 30, 90].includes(days))
    throw createError({ statusCode: 400, message: '统计时间范围无效' })
  return client.begin('isolation level repeatable read read only', async tx => {
    const since = tx`((now() at time zone 'UTC')::date - (${days}-1)) at time zone 'UTC'`
    const [visits] = await tx<
      Pick<AnalyticsReport['totals'], 'views' | 'visitors' | 'reads'>[]
    >`select count(*)::int as views,count(distinct visitor_key)::int as visitors,count(read_at)::int as reads from analytics_visits where created_at>=${since}`
    const [ads] = await tx<
      Pick<AnalyticsReport['totals'], 'impressions' | 'clicks'>[]
    >`select count(*) filter(where kind='impression')::int as impressions,count(*) filter(where kind='click')::int as clicks from ad_events e join analytics_visits v on v.id=e.visit_id where v.created_at>=${since}`
    const [accounts] = await tx<
      { count: number }[]
    >`select count(*)::int as count from users where role='user' and created_at>=${since}`
    const [completed] = await tx<
      { count: number }[]
    >`select count(*)::int as count from chapter_progress where completed_at>=${since}`
    const daily = await tx<
      AnalyticsReport['daily']
    >`select (created_at at time zone 'UTC')::date::text as day,count(*)::int as views,count(distinct visitor_key)::int as visitors,count(read_at)::int as reads from analytics_visits where created_at>=${since} group by day order by day`
    const sources = await tx<
      AnalyticsReport['sources']
    >`select source,medium,campaign,channel,count(*)::int as views,count(distinct visitor_key)::int as visitors,count(read_at)::int as reads from analytics_visits where created_at>=${since} group by source,medium,campaign,channel order by views desc,source limit 30`
    const pages = await tx<
      AnalyticsReport['pages']
    >`select path,count(*)::int as views,count(distinct visitor_key)::int as visitors,count(read_at)::int as reads from analytics_visits where created_at>=${since} group by path order by views desc,path limit 30`
    const advertisements = await tx<
      AnalyticsReport['advertisements']
    >`select ad_id as id,max(ad_name) as name,placement,count(*) filter(where kind='impression')::int as impressions,count(*) filter(where kind='click')::int as clicks from ad_events e join analytics_visits v on v.id=e.visit_id where v.created_at>=${since} group by ad_id,placement order by impressions desc limit 30`
    return {
      days,
      totals: {
        ...visits!,
        ...ads!,
        newAccounts: accounts!.count,
        completedChapters: completed!.count
      },
      daily,
      sources,
      pages,
      advertisements
    }
  })
}
