/**
 * Normalize an endpoint that is shown to users or copied into client config.
 *
 * Public gateway endpoints must use HTTPS. This intentionally does not touch
 * arbitrary upstream URLs, proxy URLs, OAuth callbacks, or internal URLs.
 */
export function normalizePublicEndpoint(value: string, fallback = ''): string {
  const candidate = value.trim() || fallback.trim()
  if (!candidate) return ''

  return candidate
    .replace(/^http:\/\//i, 'https://')
    .replace(/\/+$/, '')
}

export function getPublicOrigin(): string {
  if (typeof window === 'undefined') return ''
  return normalizePublicEndpoint(window.location.origin)
}
