import { reconcileReferencePublications } from '../services/references'
export default defineNitroPlugin(async () => {
  if (import.meta.prerender) return
  await reconcileReferencePublications()
})
