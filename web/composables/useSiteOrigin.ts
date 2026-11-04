import { useRequestURL, useState } from '#imports'
import { normalizeSiteOrigin } from '~/shared/seo'

/**
 * Canonical site origin shared by SSR and hydration.
 * Server prefers APP_BASE_URL (same source as sitemap/robots), falling back to
 * the request origin; the resolved value travels to the client via payload.
 */
export function useSiteOrigin(): string {
  const origin = useState<string>('seo:site-origin', () =>
    normalizeSiteOrigin(
      import.meta.server ? process.env.APP_BASE_URL : '',
      useRequestURL().origin
    )
  )
  return origin.value
}
