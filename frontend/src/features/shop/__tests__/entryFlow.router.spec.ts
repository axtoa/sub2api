import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { completeShopEntry, resolveShopEntryRedirect, shopEntryResultPrefix } from '../entryFlow'

const entryId = '5c58b160-81f8-416c-83e3-9fa479fd0aa8'
const initialTarget = `/shop?product=anthropic-pro&home_entry=${entryId}`

function setupRouter() {
  const session = { authenticated: false, enabled: true }
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['/home', '/login', '/register', '/email-verify', '/auth/callback', '/auth/oidc/callback', '/dashboard', '/admin/dashboard', '/shop'].map(path => ({
      path, component: { template: '<span />' }, meta: { requiresAuth: ['/shop', '/dashboard', '/admin/dashboard'].includes(path) },
    })),
  })
  router.beforeEach(to => {
    const redirect = resolveShopEntryRedirect(to, session.authenticated)
    if (redirect && redirect !== to.fullPath) {
      const destination = new URL(redirect, window.location.origin)
      return { path: destination.pathname, query: Object.fromEntries(destination.searchParams), replace: true }
    }
    if (to.meta.requiresAuth && !session.authenticated) return { path: '/login', query: { redirect: to.fullPath } }
  })
  router.afterEach((to, _from, failure) => {
    if (!failure) completeShopEntry(to.path, session.authenticated, session.enabled)
  })
  return { router, session }
}

beforeEach(() => { sessionStorage.clear(); localStorage.clear() })

describe('real Vue Router shop authentication journey', () => {
  it('requires login and keeps the intended product in the login URL', async () => {
    const { router } = setupRouter()
    await router.push(initialTarget)
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.redirect).toBe(initialTarget)
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).toBeNull()
  })

  it.each([
    ['password and two-factor', []],
    ['registration and email verification', ['/register', '/email-verify']],
    ['OAuth', ['/auth/callback']],
    ['OIDC', ['/auth/oidc/callback']],
  ] as const)('continues %s to the selected product without another click', async (_name, authPages) => {
    const { router, session } = setupRouter()
    await router.push(initialTarget)
    for (const path of authPages) await router.push(path)
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).toBeNull()
    session.authenticated = true
    await router.push('/dashboard')
    expect(router.currentRoute.value.fullPath).toBe('/shop?product=anthropic-pro')
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).not.toBeNull()
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('skips login for an existing session and strips the one-time marker', async () => {
    const { router, session } = setupRouter()
    session.authenticated = true
    await router.push(initialTarget)
    expect(router.currentRoute.value.fullPath).toBe('/shop?product=anthropic-pro')
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).not.toBeNull()
  })

  it('does not hijack ordinary login or a cancelled purchase', async () => {
    const { router, session } = setupRouter()
    await router.push(initialTarget)
    await router.push('/home')
    await router.push('/login')
    session.authenticated = true
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).toBeNull()
  })

  it('does not notify the originating tab when purchasing was disabled during login', async () => {
    const { router, session } = setupRouter()
    await router.push(initialTarget)
    session.authenticated = true
    session.enabled = false
    await router.push('/dashboard')
    expect(localStorage.getItem(shopEntryResultPrefix + entryId)).toBeNull()
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })
})
