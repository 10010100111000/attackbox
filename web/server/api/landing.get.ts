import { defineEventHandler } from 'h3'
import { listPublishedModules } from '../services/module-catalog'

// Public metadata only; chapters, answers and unpublished content stay out.
export default defineEventHandler(async () => ({ ok: true, data: await listPublishedModules(6) }))
