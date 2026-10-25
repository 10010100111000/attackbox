import {defineEventHandler,setResponseHeader} from 'h3'
export default defineEventHandler(e=>{if(e.path.startsWith('/api/admin/content'))setResponseHeader(e,'Cache-Control','private, no-store')})
