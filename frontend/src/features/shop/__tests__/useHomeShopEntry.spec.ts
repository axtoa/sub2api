import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { useHomeShopEntry } from '../useHomeShopEntry'
import { shopEntryResultPrefix } from '../entryFlow'

enableAutoUnmount(afterEach)
afterEach(() => vi.restoreAllMocks())
const { router, auth } = vi.hoisted(() => ({
  router: { currentRoute: { value: { path: '/home' } }, replace: vi.fn() },
  auth: { checkAuth: vi.fn(), refreshUser: vi.fn(), isAuthenticated: true, isAdmin: false },
}))
vi.mock('vue-router', () => ({ useRouter: () => router }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))

function setupEntry() {
  const enabled = ref(true)
  const wrapper = mount({
    setup: () => useHomeShopEntry(enabled),
    template: '<span />',
  })
  const id = wrapper.vm.entryTo().query.home_entry
  const click = () => wrapper.vm.trackEntry(new MouseEvent('click', { button: 0 }))
  function signal(idOverride = id) {
    const key = shopEntryResultPrefix + idOverride
    localStorage.setItem(key, JSON.stringify({ entryId: idOverride, completedAt: Date.now() }))
    window.dispatchEvent(new StorageEvent('storage', { key }))
  }
  return { wrapper, enabled, id, click, signal }
}

beforeEach(() => {
  vi.clearAllMocks(); localStorage.clear()
  router.currentRoute.value.path = '/home'
  auth.isAuthenticated = true; auth.isAdmin = false
  auth.refreshUser.mockResolvedValue({ role: 'user' })
  router.replace.mockResolvedValue(undefined)
})

describe('originating homepage tab', () => {
  it('keeps ordinary shop links usable if the browser cannot create a cross-tab identifier', () => {
    vi.spyOn(crypto, 'getRandomValues').mockImplementation(() => { throw new Error('unavailable') })
    const { wrapper, click } = setupEntry()
    expect(wrapper.vm.entryTo('plus')).toEqual({ path: '/shop', query: { product: 'plus' } })
    expect(click).not.toThrow()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('uses a real target destination and does not navigate or open a popup on click', () => {
    const { wrapper, click, id } = setupEntry()
    expect(wrapper.vm.entryTo('plus')).toEqual({ path: '/shop', query: { product: 'plus', home_entry: id } })
    click()
    expect(router.replace).not.toHaveBeenCalled()
    expect(auth.refreshUser).not.toHaveBeenCalled()
  })

  it('verifies the shared session and opens the user console exactly once after matching completion', async () => {
    const { click, signal, id } = setupEntry()
    click(); signal(); signal(); await flushPromises()
    expect(auth.checkAuth).toHaveBeenCalledOnce()
    expect(auth.refreshUser).toHaveBeenCalledOnce()
    expect(router.replace).toHaveBeenCalledOnce()
    expect(router.replace).toHaveBeenCalledWith('/dashboard')
    expect(localStorage.getItem(shopEntryResultPrefix + id)).toBeNull()
  })

  it('uses the admin console for a verified admin and also supports a middle click', async () => {
    auth.isAdmin = true
    const { wrapper, signal } = setupEntry()
    wrapper.vm.trackEntry(new MouseEvent('auxclick', { button: 1 }))
    signal(); await flushPromises()
    expect(router.replace).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('ignores unrelated signals and never redirects a homepage without its own click', async () => {
    const { click, signal } = setupEntry()
    signal(); await flushPromises()
    expect(auth.checkAuth).not.toHaveBeenCalled()
    click(); signal('other'); await flushPromises()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('ignores a matching storage event unless a valid completion was actually stored', async () => {
    const { click, id } = setupEntry()
    click()
    window.dispatchEvent(new StorageEvent('storage', { key: shopEntryResultPrefix + id }))
    await flushPromises()
    expect(auth.refreshUser).not.toHaveBeenCalled()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('does not treat a notification as login and retries a network failure on focus', async () => {
    auth.refreshUser.mockRejectedValueOnce(new Error('offline'))
    const { click, signal } = setupEntry()
    click(); signal(); await flushPromises()
    expect(router.replace).not.toHaveBeenCalled()
    window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(router.replace).toHaveBeenCalledWith('/dashboard')
  })

  it('does not navigate if verification clears an expired login', async () => {
    auth.refreshUser.mockImplementation(async () => { auth.isAuthenticated = false; throw new Error('401') })
    const { click, signal } = setupEntry()
    click(); signal(); await flushPromises()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('leaves the user where they navigated instead of hijacking another console page', async () => {
    const { click, signal } = setupEntry()
    click(); router.currentRoute.value.path = '/profile'; signal(); await flushPromises()
    expect(auth.refreshUser).not.toHaveBeenCalled()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('blocks new entries when the switch closes', async () => {
    const { wrapper, enabled, signal } = setupEntry()
    enabled.value = false
    const event = new MouseEvent('click', { cancelable: true })
    wrapper.vm.trackEntry(event)
    expect(event.defaultPrevented).toBe(true)
    signal(); await flushPromises()
    expect(router.replace).not.toHaveBeenCalled()
  })

  it('does not redirect or leave listeners when the original homepage is closed during verification', async () => {
    let resolve!: () => void
    auth.refreshUser.mockReturnValue(new Promise<void>(done => { resolve = done }))
    const { wrapper, click, signal } = setupEntry()
    click(); signal(); await flushPromises()
    wrapper.unmount()
    resolve(); await flushPromises()
    expect(router.replace).not.toHaveBeenCalled()
    signal(); window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(auth.refreshUser).toHaveBeenCalledOnce()
  })
})
