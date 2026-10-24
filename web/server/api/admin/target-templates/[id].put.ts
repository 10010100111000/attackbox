import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveTargetTemplate} from '../../../services/target-templates'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await saveTargetTemplate(getRouterParam(e,'id')!,await readBody(e))}})
