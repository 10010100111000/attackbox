import {defineEventHandler,getRouterParam} from 'h3'
import {eq,and} from 'drizzle-orm'
import {requireAdmin} from '../../../../utils/admin'
import {composeTemplate} from '../../../../services/compose-templates'
import {db} from '../../../../db'
import {labPreparations} from '../../../../db/schema'
import {sandboxHub} from '../../../../sandbox'
export default defineEventHandler(async e=>{requireAdmin(e);const t=await composeTemplate(getRouterParam(e,'id')!),hub=sandboxHub();return {data:{items:await db.select().from(labPreparations).where(and(eq(labPreparations.templateId,t.id),eq(labPreparations.templateVersion,t.revision))),nodes:hub.snapshot().nodes.map(n=>({id:n.id,label:n.label,bootId:n.bootId,available:n.state==='online'&&hub.composeNodes.has(n.id),state:n.state}))}}})
