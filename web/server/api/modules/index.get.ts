import { defineEventHandler } from 'h3'
import { listPublishedModules } from '../../services/module-catalog'

export default defineEventHandler(async () => ({ ok: true, data: await listPublishedModules() }))
