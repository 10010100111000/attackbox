import {defineEventHandler,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveRoute} from '../../../services/routes'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await saveRoute(null,await readBody(e))}})
