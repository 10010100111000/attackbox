import { applyResponseCachePolicy, markRenderedResponse } from '../utils/request-policy'

export default defineNitroPlugin(nitro => {
  nitro.hooks.hook('render:response', (_response, { event }) => markRenderedResponse(event))
  nitro.hooks.hook('beforeResponse', applyResponseCachePolicy)
})
