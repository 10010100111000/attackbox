import {defineEventHandler,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveChallenge} from '../../../services/challenges'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await saveChallenge(null,await readBody(e))}})
