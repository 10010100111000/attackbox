import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { importReference } from '../../../services/references'
export default defineEventHandler(async event => { const admin = requireAdmin(event); return { data: await importReference(admin.id, (await readBody(event)) || {}) } })
