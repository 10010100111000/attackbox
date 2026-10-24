import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {deleteChallenge} from '../../../services/challenges'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await deleteChallenge(getRouterParam(e,'id')!,(await readBody(e))?.lockVersion)}})
