import {defineEventHandler,getRouterParam} from 'h3'
import {requireAdmin} from '../../../../../utils/admin'
import {getContentPreview} from '../../../../../services/content-sync'
export default defineEventHandler(async e=>({data:await getContentPreview(getRouterParam(e,'id')!,requireAdmin(e).id)}))
