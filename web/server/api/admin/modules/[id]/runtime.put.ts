import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../../utils/admin'
import {saveModuleRuntime} from '../../../../services/module-runtime-settings'
export default defineEventHandler(async e=>{const admin=requireAdmin(e);return {data:await saveModuleRuntime(getRouterParam(e,'id')!,admin.id,(await readBody(e))||{})}})
