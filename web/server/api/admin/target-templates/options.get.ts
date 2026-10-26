import {defineEventHandler} from 'h3'
import {asc,eq} from 'drizzle-orm'
import {db} from '../../../db'
import {targetTemplates} from '../../../db/schema'
import {requireAdmin} from '../../../utils/admin'
export default defineEventHandler(async event=>{requireAdmin(event);return {ok:true,data:await db.select({id:targetTemplates.id,name:targetTemplates.name,enabled:targetTemplates.enabled}).from(targetTemplates).where(eq(targetTemplates.runtime,'compose')).orderBy(asc(targetTemplates.name),targetTemplates.id)}})
