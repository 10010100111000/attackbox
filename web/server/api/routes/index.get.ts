import { defineEventHandler } from 'h3'
import { routeList } from '../../services/routes'
export default defineEventHandler(async () => ({ ok: true, data: await routeList() }))
