import {defineEventHandler,getRouterParam} from 'h3'
import {requireAdmin} from '../../../../../utils/admin'
import {deleteContentPreview} from '../../../../../services/content-sync'
export default defineEventHandler(async e=>({data:await deleteContentPreview(getRouterParam(e,'id')!,requireAdmin(e).id)}))
