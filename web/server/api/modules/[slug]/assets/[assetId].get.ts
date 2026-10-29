import { defineEventHandler, getRouterParam, setResponseHeaders, createError } from 'h3'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { readableContentAsset } from '../../../../services/content-images'
import { contentDataDirectory } from '../../../../content/storage'
import { activeLearner } from '../../../../utils/learner'
export default defineEventHandler(async event => {
  const id = getRouterParam(event, 'assetId') || ''
  if (!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(id))
    throw createError({ statusCode: 404, message: '资源不存在' })
  const asset = await readableContentAsset(
    getRouterParam(event, 'slug')!,
    id,
    activeLearner(event),
    event.context.user?.role === 'admin' && event.context.user.status === 'active'
  )
  if (!/^[a-f0-9]{64}$/.test(asset.storageKey))
    throw createError({ statusCode: 404, message: '资源不存在' })
  setResponseHeaders(event, {
    'Content-Type': asset.mediaType,
    'X-Content-Type-Options': 'nosniff'
  })
  try {
    return await readFile(path.join(contentDataDirectory(), 'assets', asset.storageKey))
  } catch {
    throw createError({ statusCode: 404, message: '资源文件不存在' })
  }
})
