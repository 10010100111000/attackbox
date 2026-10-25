import {defineEventHandler,getRouterParam} from 'h3'
import {requireAdmin} from '../../../../../utils/admin'
import {applyContentPreview} from '../../../../../services/content-sync'
export default defineEventHandler(async e=>({data:await applyContentPreview(getRouterParam(e,'id')!,requireAdmin(e).id)}))
