import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { getRuntimePolicy } from '../../../services/runtime-policy'
export default defineEventHandler(async (e) => {
  requireAdmin(e)
  return { data: await getRuntimePolicy() }
})
