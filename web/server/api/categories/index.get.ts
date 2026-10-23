import { defineEventHandler } from 'h3'
import { asc } from 'drizzle-orm'
import { db } from '../../db'
import { contentCategories } from '../../db/schema'
export default defineEventHandler(async () => ({ ok: true, data: await db.select().from(contentCategories).orderBy(asc(contentCategories.position), asc(contentCategories.name)) }))
