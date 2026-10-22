import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { completeShopEntry, createShopEntryId, hasShopEntryResult, resolveShopEntryRedirect, shopDestination, shopEntryLifetime, shopEntryResultPrefix } from '../entryFlow'

const entryId = 'a6300db2-93a0-4fb8-ac93-6957820a43ae'
const intentKey = 'best-ai:shop-entry-intent'
function route(fullPath: string) {
  const url = new URL(fullPath, window.location.origin)
  return { path: url.pathname, fullPath, query: Object.fromEntries(url.searchParams) }
}
function enter(product = 'chatgpt-plus') {
  return resolveShopEntryRedirect(route(`${shopDestination(product)}&home_entry=${entryId}`), false)
}

beforeEach(() => { sessionStorage.clear(); localStorage.clear() })
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

describe('shop-only authentication continuation', () => {
  it('creates distinct UUIDs without depending on the secure-context-only randomUUID API', () => {
    vi.spyOn(crypto, 'randomUUID').mockImplementation(() => { throw new Error('HTTP origin') })
    const id = createShopEntryId()
    expect(id).toMatch(/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/)
    expect(createShopEntryId()).not.toBe(id)
  })

  it('does not change ordinary login, registration or dashboard navigation', () => {
    for (const path of ['/login', '/register', '/email-verify', '/dashboard', '/admin/dashboard', '/auth/callback']) {
      expect(resolveShopEntryRedirect(route(path), true)).toBeUndefined()
    }
    completeShopEntry('/shop', true, true)
    expect(localStorage.length).toBe(0)
  })

  it('preserves the selected product through login, registration and email verification, exactly once', () => {
    const startedAt = Date.now()
    enter()
    for (const path of ['/login', '/register', '/email-verify']) {
      expect(resolveShopEntryRedirect(route(path), false)).toBeUndefined()
    }
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBe('/shop?product=chatgpt-plus')
    resolveShopEntryRedirect(route('/shop?product=chatgpt-plus'), true)
    completeShopEntry('/shop', true, true)
    expect(hasShopEntryResult(entryId, startedAt)).toBe(true)
    expect(JSON.parse(localStorage.getItem(shopEntryResultPrefix + entryId)!)).toEqual({ entryId, completedAt: expect.any(Number) })
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBeUndefined()
    expect(sessionStorage.getItem(intentKey)).toBeNull()
  })

  it.each(['/auth/callback', '/auth/linuxdo/callback', '/auth/oidc/callback', '/auth/dingtalk/email-completion', '/auth/wechat/callback'])('survives %s and its default callback destination', path => {
    enter('anthropic-pro')
    resolveShopEntryRedirect(route(path), false)
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBe('/shop?product=anthropic-pro')
  })

  it('keeps the intent while password login waits for two-factor verification', () => {
    enter()
    resolveShopEntryRedirect(route('/login'), false)
    completeShopEntry('/login', false, true)
    expect(hasShopEntryResult(entryId, 0)).toBe(false)
    expect(resolveShopEntryRedirect(route('/shop?product=chatgpt-plus&home_entry=' + entryId), true)).toBe('/shop?product=chatgpt-plus')
    completeShopEntry('/shop', true, true)
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).not.toBeNull()
  })

  it('skips login for authenticated users, cleans the URL and sends a one-time completion', () => {
    expect(resolveShopEntryRedirect(route(`/shop?product=plus&home_entry=${entryId}`), true)).toBe('/shop?product=plus')
    resolveShopEntryRedirect(route('/shop?product=plus'), true)
    completeShopEntry('/shop', true, true)
    const first = localStorage.getItem(shopEntryResultPrefix + entryId)
    completeShopEntry('/shop', true, true)
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).toBe(first)
  })

  it('also preserves direct shop links across registration without redirecting any other tab', () => {
    resolveShopEntryRedirect(route('/shop/?product=plus'), false)
    resolveShopEntryRedirect(route('/register'), false)
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBe('/shop?product=plus')
    completeShopEntry('/shop', true, true)
    expect(localStorage.length).toBe(0)
  })

  it.each(['/home', '/help-center', '/profile', '/shop/orders'])('cancels a pending purchase when the user leaves for %s', path => {
    enter()
    resolveShopEntryRedirect(route(path), false)
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBeUndefined()
  })

  it('does not notify on a failed login, an admin route or a closed shop', () => {
    enter()
    completeShopEntry('/shop', false, true)
    completeShopEntry('/admin/shop', true, true)
    expect(localStorage.length).toBe(0)
    completeShopEntry('/shop', true, false)
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.getItem(intentKey)).toBeNull()
  })

  it('expires an abandoned login rather than hijacking a later ordinary login', () => {
    vi.useFakeTimers()
    enter()
    vi.advanceTimersByTime(shopEntryLifetime + 1)
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBeUndefined()
  })

  it.each(['https://evil.example/shop', '//evil.example/shop', '/admin/dashboard', '/shop/orders/other'])('rejects a tampered destination %s', destination => {
    sessionStorage.setItem(intentKey, JSON.stringify({ destination, entryId, createdAt: Date.now() }))
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBeUndefined()
    expect(sessionStorage.getItem(intentKey)).toBeNull()
  })

  it('ignores malformed storage and home markers and encodes product input as data', () => {
    sessionStorage.setItem(intentKey, 'not-json')
    expect(resolveShopEntryRedirect(route('/dashboard'), true)).toBeUndefined()
    resolveShopEntryRedirect(route('/shop?home_entry=untrusted'), false)
    completeShopEntry('/shop', true, true)
    expect(localStorage.length).toBe(0)
    expect(shopDestination('x&redirect=https://evil.example')).toBe('/shop?product=x%26redirect%3Dhttps%3A%2F%2Fevil.example')
  })

  it('does not throw when storage is unavailable', () => {
    for (const storage of [sessionStorage, localStorage]) {
      vi.spyOn(storage, 'getItem').mockImplementation(() => { throw new Error('storage disabled') })
      vi.spyOn(storage, 'setItem').mockImplementation(() => { throw new Error('storage disabled') })
      vi.spyOn(storage, 'removeItem').mockImplementation(() => { throw new Error('storage disabled') })
    }
    expect(() => enter()).not.toThrow()
    expect(() => completeShopEntry('/shop', true, true)).not.toThrow()
    expect(hasShopEntryResult(entryId, Date.now())).toBe(false)
  })

  it('rejects another tab id, stale, malformed and future completion messages', () => {
    const now = Date.now()
    for (const result of [null, {}, { entryId: 'other', completedAt: now }, { entryId, completedAt: now - 100 }, { entryId, completedAt: now + 10000 }]) {
      localStorage.setItem(shopEntryResultPrefix + entryId, JSON.stringify(result))
      expect(hasShopEntryResult(entryId, now)).toBe(false)
    }
  })

  it('prunes only expired shop notifications and never touches the shared login credentials', () => {
    const expired = shopEntryResultPrefix + 'expired'
    localStorage.setItem('auth_token', 'existing-session-token')
    localStorage.setItem('refresh_token', 'existing-refresh-token')
    localStorage.setItem(expired, JSON.stringify({ completedAt: Date.now() - shopEntryLifetime - 1 }))
    enter()
    completeShopEntry('/shop', true, true)
    expect(localStorage.getItem(expired)).toBeNull()
    expect(localStorage.getItem('auth_token')).toBe('existing-session-token')
    expect(localStorage.getItem('refresh_token')).toBe('existing-refresh-token')
  })
})
