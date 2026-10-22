import type { LocationQuery } from 'vue-router'

const intentKey = 'best-ai:shop-entry-intent'
export const shopEntryResultPrefix = 'best-ai:shop-entry-complete:'
export const shopEntryLifetime = 60 * 60 * 1000
const entryIdPattern = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i

interface ShopEntryIntent {
  destination: string
  entryId?: string
  createdAt: number
}

interface EntryRoute {
  path: string
  fullPath: string
  query: LocationQuery
}

export function createShopEntryId(): string | undefined {
  try {
    // Unlike randomUUID, getRandomValues is also available on HTTP preview origins.
    const bytes = crypto.getRandomValues(new Uint8Array(16))
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  } catch { return undefined }
}

export function shopDestination(product?: unknown): string {
  return typeof product === 'string' && product.length > 0 && product.length <= 128
    ? `/shop?${new URLSearchParams({ product })}`
    : '/shop'
}

function clearIntent() {
  try { sessionStorage.removeItem(intentKey) } catch { /* Authentication still works without an intent. */ }
}

function readIntent(): ShopEntryIntent | undefined {
  try {
    const raw = sessionStorage.getItem(intentKey)
    if (!raw) return
    const value = JSON.parse(raw) as ShopEntryIntent
    const url = new URL(value.destination, window.location.origin)
    if (url.origin !== window.location.origin || url.pathname !== '/shop' ||
        typeof value.createdAt !== 'number' || !Number.isFinite(value.createdAt) ||
        Date.now() - value.createdAt > shopEntryLifetime || value.createdAt > Date.now() ||
        (value.entryId !== undefined && !entryIdPattern.test(value.entryId))) {
      clearIntent()
      return
    }
    return { destination: shopDestination(url.searchParams.get('product')), entryId: value.entryId, createdAt: value.createdAt }
  } catch {
    clearIntent()
  }
}

function saveIntent(intent: ShopEntryIntent) {
  try { sessionStorage.setItem(intentKey, JSON.stringify(intent)) } catch { /* Normal login redirect remains available. */ }
}

/** Only a shop visit creates an intent. No login view or OAuth provider is changed. */
export function resolveShopEntryRedirect(to: EntryRoute, authenticated: boolean): string | undefined {
  const pending = readIntent()
  if (to.path === '/shop' || to.path === '/shop/') {
    const entryId = typeof to.query.home_entry === 'string' && entryIdPattern.test(to.query.home_entry)
      ? to.query.home_entry : undefined
    const destination = shopDestination(to.query.product)
    if (!authenticated || entryId || pending) {
      saveIntent({
        destination,
        entryId: entryId ?? pending?.entryId,
        createdAt: entryId && entryId !== pending?.entryId ? Date.now() : pending?.createdAt ?? Date.now(),
      })
    }
    // Remove the one-time marker before displaying the catalogue or sharing its URL.
    if (authenticated && to.query.home_entry !== undefined) return destination
    return
  }

  if (!pending) return
  if (authenticated && ['/dashboard', '/admin/dashboard', '/login', '/register'].includes(to.path)) {
    return pending.destination
  }
  const inAuthJourney = ['/login', '/register', '/email-verify', '/forgot-password', '/reset-password', '/dashboard', '/admin/dashboard'].includes(to.path)
    || to.path.startsWith('/auth/')
  if (!inAuthJourney) clearIntent()
}

/** Called only after a successful protected catalogue navigation. Notifications contain no identity or token. */
export function completeShopEntry(path: string, authenticated: boolean, enabled: boolean) {
  if (!authenticated || (path !== '/shop' && path !== '/shop/')) return
  const pending = readIntent()
  clearIntent()
  if (!enabled || !pending?.entryId) return
  try {
    // Bound orphaned notifications if the originating tab was already closed.
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i)
      if (!key?.startsWith(shopEntryResultPrefix)) continue
      try {
        const result = JSON.parse(localStorage.getItem(key) || 'null')
        if (!result || typeof result.completedAt !== 'number' || Date.now() - result.completedAt > shopEntryLifetime) {
          localStorage.removeItem(key)
        }
      } catch { localStorage.removeItem(key) }
    }
    localStorage.setItem(shopEntryResultPrefix + pending.entryId, JSON.stringify({ entryId: pending.entryId, completedAt: Date.now() }))
  } catch { /* The shop tab remains usable even if another tab cannot be notified. */ }
}

export function hasShopEntryResult(entryId: string, startedAt: number): boolean {
  try {
    const result = JSON.parse(localStorage.getItem(shopEntryResultPrefix + entryId) || 'null')
    return result?.entryId === entryId && typeof result.completedAt === 'number'
      && Number.isFinite(result.completedAt) && result.completedAt >= startedAt
      && result.completedAt <= Date.now() && Date.now() - startedAt <= shopEntryLifetime
  } catch { return false }
}

export function removeShopEntryResult(entryId: string) {
  try { localStorage.removeItem(shopEntryResultPrefix + entryId) } catch { /* Best-effort cleanup. */ }
}
