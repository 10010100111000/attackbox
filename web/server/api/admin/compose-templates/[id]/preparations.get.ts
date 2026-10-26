import {defineEventHandler,getRouterParam,setResponseHeader} from 'h3'
import {eq,and} from 'drizzle-orm'
import {requireAdmin} from '../../../../utils/admin'
import {composeTemplate} from '../../../../services/compose-templates'
import {db} from '../../../../db'
import {labPreparations} from '../../../../db/schema'
import {sandboxHub} from '../../../../sandbox'
export default defineEventHandler(async e=>{requireAdmin(e);setResponseHeader(e,'Cache-Control','private, no-store');const t=await composeTemplate(getRouterParam(e,'id')!),hub=sandboxHub();return {data:{items:await db.select().from(labPreparations).where(and(eq(labPreparations.templateId,t.id),eq(labPreparations.templateVersion,t.composeRevision))),nodes:hub.snapshot().nodes.map(n=>({id:n.id,label:n.label,available:n.state==='online'&&hub.composeNodes.has(n.id),state:n.state}))}}})
