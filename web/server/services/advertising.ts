import { randomInt } from 'node:crypto'
import { createError } from 'h3'
import { client } from '../db'
import {
  parseAd,
  isAdPlacement,
  type Advertisement,
  type AdInput,
  type AdCreative
} from '../../shared/advertising'
import { analyticsId } from '../../shared/analytics'

const columns = client`id, name, title, description, image_url as "imageUrl", href, cta, sponsor, placements, enabled, weight, starts_at as "startsAt", ends_at as "endsAt", lock_version as "lockVersion"`
function output(row: Record<string, unknown>): Advertisement {
  return {
    ...row,
    startsAt: row.startsAt instanceof Date ? row.startsAt.toISOString() : null,
    endsAt: row.endsAt instanceof Date ? row.endsAt.toISOString() : null
  } as Advertisement
}
export function inputAd(body: unknown) {
  try {
    return parseAd(body)
  } catch (error) {
    throw createError({ statusCode: 400, message: (error as Error).message })
  }
}
export function adId(value: unknown) {
  try {
    return analyticsId(value)
  } catch {
    throw createError({ statusCode: 400, message: '广告标识无效' })
  }
}
export async function listAdvertisements(page: number, search: string) {
  if (!Number.isSafeInteger(page) || page < 1 || page > 100000 || search.length > 120)
    throw createError({ statusCode: 400, message: '查询条件无效' })
  return client.begin('isolation level repeatable read read only', async tx => {
    const where = tx`(strpos(lower(name || ' ' || title),lower(${search}))>0)`
    const [count] = await tx`select count(*)::int as total from advertisements where ${where}`
    const rows =
      await tx`select ${columns} from advertisements where ${where} order by updated_at desc,id limit 20 offset ${(page - 1) * 20}`
    const ids = rows.map(row => row.id)
    const stats = ids.length
      ? await tx`select ad_id as id, count(*) filter(where kind='impression')::int as impressions, count(*) filter(where kind='click')::int as clicks from ad_events e join analytics_visits v on v.id=e.visit_id where ad_id=any(${ids}::uuid[]) and v.created_at>=now()-interval '30 days' group by ad_id`
      : []
    return {
      items: rows.map(row => ({
        ...output(row),
        impressions: stats.find(stat => stat.id === row.id)?.impressions || 0,
        clicks: stats.find(stat => stat.id === row.id)?.clicks || 0
      })),
      total: count!.total,
      page,
      pageSize: 20
    }
  })
}
export async function getAdvertisement(id: string) {
  const [row] = await client`select ${columns} from advertisements where id=${adId(id)}`
  if (!row) throw createError({ statusCode: 404, message: '广告不存在' })
  return output(row)
}
export async function saveAdvertisement(id: string | null, ad: AdInput) {
  if ((!id && ad.lockVersion !== 0) || (id && ad.lockVersion === 0))
    throw createError({ statusCode: 400, message: '广告版本无效' })
  const values = {
    name: ad.name,
    title: ad.title,
    description: ad.description,
    image_url: ad.imageUrl,
    href: ad.href,
    cta: ad.cta,
    sponsor: ad.sponsor,
    placements: ad.placements,
    enabled: ad.enabled,
    weight: ad.weight,
    starts_at: ad.startsAt,
    ends_at: ad.endsAt,
    lock_version: ad.lockVersion + 1,
    updated_at: new Date().toISOString()
  }
  const rows = id
    ? await client`update advertisements set ${client(values)} where id=${adId(id)} and lock_version=${ad.lockVersion} returning ${columns}`
    : await client`insert into advertisements ${client(values)} returning ${columns}`
  if (!rows[0]) throw createError({ statusCode: 409, message: '广告已被修改或删除，请重新载入' })
  return output(rows[0])
}
export async function deleteAdvertisement(id: string, version: unknown) {
  if (!Number.isSafeInteger(version) || Number(version) < 1)
    throw createError({ statusCode: 400, message: '广告版本无效' })
  const rows =
    await client`delete from advertisements where id=${adId(id)} and lock_version=${Number(version)} returning id`
  if (!rows.length)
    throw createError({ statusCode: 409, message: '广告已被修改或删除，请刷新列表' })
}
export async function selectAdvertisement(placement: unknown): Promise<AdCreative | null> {
  if (!isAdPlacement(placement)) throw createError({ statusCode: 400, message: '广告位无效' })
  const rows =
    await client`select id,title,description,image_url as "imageUrl",href,cta,sponsor,weight from advertisements where enabled and ${placement}=any(placements) and (starts_at is null or starts_at<=now()) and (ends_at is null or ends_at>now())`
  if (!rows.length) return null
  let draw = randomInt(rows.reduce((sum, row) => sum + row.weight, 0))
  for (const row of rows) {
    draw -= row.weight
    if (draw < 0) {
      const { weight: _weight, ...creative } = row
      return creative as AdCreative
    }
  }
  return null
}
