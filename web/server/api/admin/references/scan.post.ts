import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { scanReferences } from '../../../services/references'
export default defineEventHandler(async event => { requireAdmin(event); return { data: await scanReferences() } })
