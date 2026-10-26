import {defineEventHandler,readBody,getRouterParam} from 'h3'
import {requireAdmin} from '../../../../utils/admin'
import {labGitAction} from '../../../../services/lab-git'
export default defineEventHandler(async e=>{const a=requireAdmin(e);return {data:await labGitAction(a.id,await readBody(e),getRouterParam(e,'id')!)}})
