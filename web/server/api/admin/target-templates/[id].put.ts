import {defineEventHandler,createError} from 'h3'
import {requireAdmin} from '../../../utils/admin'
export default defineEventHandler(e=>{requireAdmin(e);throw createError({statusCode:410,message:'旧单容器接口已退出，请使用 Compose 部署包与节点准备接口'})})
