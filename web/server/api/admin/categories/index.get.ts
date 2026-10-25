import {defineEventHandler} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {adminCategories} from '../../../services/categories'
export default defineEventHandler(event=>{requireAdmin(event);return adminCategories().then(data=>({data}))})
