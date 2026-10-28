import { defineEventHandler, getRouterParam, setResponseHeaders, createError } from 'h3'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { referenceAsset } from '../../../../../services/references'
import { contentDataDirectory } from '../../../../../content/storage'
import { requireAdmin } from '../../../../../utils/admin'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const asset = await referenceAsset(getRouterParam(event, 'id') || '', getRouterParam(event, 'assetId') || '', true)
  if (!/^[a-f0-9]{64}$/.test(asset.storageKey)) throw createError({statusCode:404,message:'图片不存在'})
  setResponseHeaders(event, {'Content-Type':asset.mediaType,'X-Content-Type-Options':'nosniff'})
  try { return await readFile(path.join(contentDataDirectory(),'assets',asset.storageKey)) }
  catch { throw createError({statusCode:404,message:'图片不存在'}) }
})
