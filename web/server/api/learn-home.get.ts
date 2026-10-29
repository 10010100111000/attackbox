import { defineEventHandler } from 'h3'
import { getLearnHome } from '../services/learn-home'
export default defineEventHandler(async () => {
  const config = await getLearnHome()
  return {
    data: {
      enabled: config.enabled,
      autoplay: config.autoplay,
      intervalSeconds: config.intervalSeconds,
      slides: config.enabled ? config.slides.filter(s => s.enabled) : []
    }
  }
})
