import { defineEventHandler, getRouterParam } from 'h3'
import { readReference } from '../../../content/reference-files'
export default defineEventHandler(async event => ({
  data: await readReference(getRouterParam(event, 'slug') || '')
}))
