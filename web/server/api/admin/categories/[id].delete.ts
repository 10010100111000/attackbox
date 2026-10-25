import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {deleteCategory} from '../../../services/categories'
export default defineEventHandler(async event=>{requireAdmin(event);const body=await readBody(event);return {data:await deleteCategory(getRouterParam(event,'id')||'',body?.currentName,body?.moduleCount)}})
