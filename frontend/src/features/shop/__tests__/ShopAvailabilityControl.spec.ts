import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import ShopAvailabilityControl from '../ShopAvailabilityControl.vue'
import { shopAPI } from '../api'
import { useShopAvailability } from '../availability'

vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', () => ({ shopAPI: { settings: vi.fn(), saveSettings: vi.fn(), externalSettings: vi.fn(), saveExternalSettings: vi.fn() } }))

let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  setActivePinia(createPinia())
  vi.mocked(shopAPI.settings).mockResolvedValue({ enabled: true })
  vi.mocked(shopAPI.saveSettings).mockImplementation(async enabled => ({ enabled }))
  vi.mocked(shopAPI.externalSettings).mockResolvedValue({ enabled: false, url: 'https://wzyp.cn/shop/best-ai' })
  vi.mocked(shopAPI.saveExternalSettings).mockImplementation(async settings => settings)
})
afterEach(() => wrapper?.unmount())

describe('official purchasing availability', () => {
  it('waits for the server state and disables input during loading', async () => {
    let resolve!: (value: { enabled: boolean }) => void
    vi.mocked(shopAPI.settings).mockReturnValueOnce(new Promise(done => { resolve = done }))
    wrapper = mount(ShopAvailabilityControl)
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeDefined()
    resolve({ enabled: false }); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.text()).toContain('shop.availabilityOff')
  })
  it('only changes state after a successful save and prevents double submission', async () => {
    const app = useAppStore()
    app.cachedPublicSettings = { site_name: 'Best AI' } as NonNullable<typeof app.cachedPublicSettings>
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    let resolve!: (value: { enabled: boolean }) => void
    vi.mocked(shopAPI.saveSettings).mockReturnValueOnce(new Promise(done => { resolve = done }))
    const toggle = wrapper.get('[role="switch"]')
    await toggle.trigger('click'); await toggle.trigger('click')
    expect(shopAPI.saveSettings).toHaveBeenCalledTimes(1)
    expect(shopAPI.saveSettings).toHaveBeenCalledWith(false)
    expect(toggle.attributes('aria-checked')).toBe('true')
    expect(toggle.attributes('disabled')).toBeDefined()
    resolve({ enabled: false }); await flushPromises()
    expect(toggle.attributes('aria-checked')).toBe('false')
    expect(app.cachedPublicSettings.official_shop_enabled).toBe(false)
    expect(localStorage.getItem('best-ai:official-shop-change')).not.toBeNull()
    await toggle.trigger('click'); await flushPromises()
    expect(toggle.attributes('aria-checked')).toBe('true')
    expect(shopAPI.saveSettings).toHaveBeenLastCalledWith(true)
  })
  it('keeps the prior state and allows retry after a save error', async () => {
    vi.mocked(shopAPI.saveSettings).mockRejectedValueOnce(new Error('database unavailable'))
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[role="alert"]').text()).toContain('database unavailable')
    expect(localStorage.getItem('best-ai:official-shop-change')).toBeNull()
    await wrapper.get('[role="switch"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
  })
  it('does not guess the state if loading fails and offers retry', async () => {
    vi.mocked(shopAPI.settings).mockRejectedValueOnce(new Error('offline'))
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('shop.availabilityUnknown')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeUndefined()
  })
  it('refreshes authoritative public settings on another tab change or focus and removes listeners on unmount', async () => {
    const app = useAppStore()
    const refresh = vi.spyOn(app, 'fetchPublicSettings').mockResolvedValue(null)
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' }))
    expect(refresh).not.toHaveBeenCalled()
    window.dispatchEvent(new StorageEvent('storage', { key: 'best-ai:official-shop-change' }))
    window.dispatchEvent(new Event('focus'))
    expect(refresh).toHaveBeenCalledTimes(2)
    expect(refresh).toHaveBeenLastCalledWith(true)
    wrapper.unmount(); wrapper = undefined
    window.dispatchEvent(new Event('focus'))
    expect(refresh).toHaveBeenCalledTimes(2)
  })
  it('keeps the toggle aligned with a refreshed state from another admin tab', async () => {
    const app = useAppStore()
    app.cachedPublicSettings = { official_shop_enabled: true } as NonNullable<typeof app.cachedPublicSettings>
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    app.cachedPublicSettings.official_shop_enabled = false; await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(shopAPI.saveSettings).not.toHaveBeenCalled()
  })
  it('honors a closed server response even before public configuration has loaded', async () => {
    const app = useAppStore()
    expect(app.cachedPublicSettings).toBeNull()
    const probe = mount({ setup: useShopAvailability, template: '<span>{{ enabled }}</span>' })
    wrapper = probe
    probe.vm.apply(false)
    await flushPromises()
    expect(probe.text()).toBe('false')
    app.cachedPublicSettings = { official_shop_enabled: true } as NonNullable<typeof app.cachedPublicSettings>
    await flushPromises()
    expect(probe.text()).toBe('true')
  })

  it('keeps the external marketplace switch independent from the on-site shop', async () => {
    const app = useAppStore()
    vi.mocked(shopAPI.settings).mockResolvedValueOnce({ enabled: false })
    app.cachedPublicSettings = { official_shop_enabled: false } as NonNullable<typeof app.cachedPublicSettings>
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    await wrapper.get('[aria-label="shop.externalAvailability"][role="switch"]').trigger('click')
    await flushPromises()
    expect(shopAPI.saveExternalSettings).toHaveBeenCalledWith({ enabled: true, url: 'https://wzyp.cn/shop/best-ai' })
    expect(shopAPI.saveSettings).not.toHaveBeenCalled()
    expect(app.cachedPublicSettings.official_shop_enabled).toBe(false)
    expect(app.cachedPublicSettings.external_shop_enabled).toBe(true)
  })

  it('does not publish an invalid external URL', async () => {
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    await wrapper.get('#external-shop-url').setValue('javascript:alert(1)')
    await wrapper.get('button.shop-secondary').trigger('click')
    expect(shopAPI.saveExternalSettings).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('shop.externalInvalidURL')
  })
})
