import {defineEventHandler} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {testContentRepository} from '../../../services/content-repository'
export default defineEventHandler(async e=>{requireAdmin(e);return {data:await testContentRepository()}})
