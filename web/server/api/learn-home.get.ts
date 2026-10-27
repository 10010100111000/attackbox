import { defineEventHandler, setResponseHeader } from 'h3'
import { getLearnHome } from '../services/learn-home'
export default defineEventHandler(async e => {
 setResponseHeader(e,'Cache-Control','no-store')
 const config=await getLearnHome()
 return { data: {enabled:config.enabled,autoplay:config.autoplay,intervalSeconds:config.intervalSeconds,slides:config.enabled ? config.slides.filter(s=>s.enabled) : []} }
})
