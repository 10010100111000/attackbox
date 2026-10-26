import {defineEventHandler,setResponseHeader} from 'h3'
export default defineEventHandler(e=>{if(['/api/admin/content','/api/admin/compose-templates','/api/admin/lab-bundles'].some(p=>e.path.startsWith(p)))setResponseHeader(e,'Cache-Control','private, no-store')})
