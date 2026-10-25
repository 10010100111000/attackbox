import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../../../utils/admin'
import {updateContentBindings} from '../../../../../services/content-sync'
export default defineEventHandler(async e=>{const admin=requireAdmin(e);return {data:await updateContentBindings(getRouterParam(e,'id')!,admin.id,(await readBody(e))?.bindings)}})
