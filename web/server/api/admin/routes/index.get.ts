import {defineEventHandler} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {routeList} from '../../../services/routes'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await routeList(true)}})
