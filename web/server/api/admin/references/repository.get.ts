import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { contentRepositorySettings } from '../../../services/content-repository'
export default defineEventHandler(async event => { requireAdmin(event); return { data: await contentRepositorySettings(undefined, 2) } })
