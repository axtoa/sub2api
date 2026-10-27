import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import ShopAvailabilityControl from '../ShopAvailabilityControl.vue'
import { shopAPI } from '../api'
import { useShopAvailability } from '../availability'

vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', () => ({ shopAPI: { settings: vi.fn(), saveSettings: vi.fn(), externalSettings: vi.fn(), saveExternalSettings: vi.fn(), labels: vi.fn(), saveLabel: vi.fn() } }))

let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  setActivePinia(createPinia())
  vi.mocked(shopAPI.settings).mockResolvedValue({ enabled: true })
  vi.mocked(shopAPI.saveSettings).mockImplementation(async enabled => ({ enabled }))
  vi.mocked(shopAPI.externalSettings).mockResolvedValue({ enabled: false, url: 'https://wzyp.cn/shop/best-ai' })
  vi.mocked(shopAPI.saveExternalSettings).mockImplementation(async settings => settings)
  vi.mocked(shopAPI.labels).mockResolvedValue({ official_name: '', external_name: '' })
  vi.mocked(shopAPI.saveLabel).mockImplementation(async (_channel, name) => ({ name }))
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
  it('omits redundant open labels and the URL heading while keeping the external URL visible', async () => {
    vi.mocked(shopAPI.externalSettings).mockResolvedValueOnce({ enabled: true, url: 'https://wzyp.cn/shop/best-ai' })
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    expect(wrapper.text()).not.toContain('shop.availabilityOn')
    expect(wrapper.text()).not.toContain('shop.externalAvailabilityOn')
    expect(wrapper.findAll('[role="status"]')).toHaveLength(0)
    expect(wrapper.find('.shop-field-label').exists()).toBe(false)
    expect(wrapper.get('.shop-readonly-value').text()).toBe('https://wzyp.cn/shop/best-ai')
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
    await wrapper.get('[aria-label="shop.editURL"]').trigger('click')
    await wrapper.get('#external-shop-url').setValue('javascript:alert(1)')
    await wrapper.get('#external-shop-url').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(shopAPI.saveExternalSettings).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('shop.externalInvalidURL')
  })

  it('shows the saved URL until editing, cancels drafts, and saves explicitly', async () => {
    const app = useAppStore()
    app.cachedPublicSettings = { site_name: 'Best AI' } as NonNullable<typeof app.cachedPublicSettings>
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    expect(wrapper.find('#external-shop-url').exists()).toBe(false)
    expect(wrapper.text()).toContain('https://wzyp.cn/shop/best-ai')
    await wrapper.get('[aria-label="shop.editURL"]').trigger('click')
    await wrapper.get('#external-shop-url').setValue('https://example.com/new')
    expect(wrapper.get('[aria-label="shop.externalAvailability"][role="switch"]').attributes('disabled')).toBeDefined()
    expect(shopAPI.saveExternalSettings).not.toHaveBeenCalled()
    await wrapper.get('#external-shop-url').element.closest('form')!.querySelector('button[type="button"]')!.dispatchEvent(new Event('click', { bubbles: true }))
    await flushPromises()
    expect(wrapper.find('#external-shop-url').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('https://example.com/new')
    await wrapper.get('[aria-label="shop.editURL"]').trigger('click')
    await wrapper.get('#external-shop-url').setValue('https://example.com/new')
    await wrapper.get('#external-shop-url').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(shopAPI.saveExternalSettings).toHaveBeenCalledWith({ enabled: false, url: 'https://example.com/new' })
    expect(app.cachedPublicSettings.external_shop_url).toBe('https://example.com/new')
    expect(wrapper.find('#external-shop-url').exists()).toBe(false)
  })

  it('keeps URL editor open and the old link active when save fails', async () => {
    const app = useAppStore()
    app.cachedPublicSettings = { site_name: 'Best AI' } as NonNullable<typeof app.cachedPublicSettings>
    vi.mocked(shopAPI.saveExternalSettings).mockRejectedValueOnce(new Error('offline'))
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    await wrapper.get('[aria-label="shop.editURL"]').trigger('click')
    await wrapper.get('#external-shop-url').setValue('https://example.com/new')
    await wrapper.get('#external-shop-url').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(wrapper.find('#external-shop-url').exists()).toBe(true)
    expect(wrapper.text()).toContain('offline')
    expect(app.cachedPublicSettings.external_shop_url).toBe('https://wzyp.cn/shop/best-ai')
  })

  it('saves each menu name independently and resets an empty name to the default', async () => {
    const app = useAppStore()
    app.cachedPublicSettings = { site_name: 'Best AI' } as NonNullable<typeof app.cachedPublicSettings>
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    expect(wrapper.find('#official-shop-name').exists()).toBe(false)
    await wrapper.get('[aria-label="shop.editName: shop.entry"]').trigger('click')
    await wrapper.get('#official-shop-name').setValue('站内精选')
    await wrapper.get('#official-shop-name').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(shopAPI.saveLabel).toHaveBeenCalledWith('official', '站内精选')
    expect(app.cachedPublicSettings.official_shop_name).toBe('站内精选')
    expect(wrapper.text()).toContain('站内精选')
    await wrapper.get('[aria-label="shop.editName: 站内精选"]').trigger('click')
    await wrapper.get('#official-shop-name').setValue('')
    await wrapper.get('#official-shop-name').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(app.cachedPublicSettings.official_shop_name).toBe('')
    expect(wrapper.get('section').get('h2').text()).toBe('shop.entry')
    expect(shopAPI.saveSettings).not.toHaveBeenCalled()
    expect(shopAPI.saveExternalSettings).not.toHaveBeenCalled()
  })

  it('does not publish an unsaved name and retains the editor after an error', async () => {
    vi.mocked(shopAPI.saveLabel).mockRejectedValueOnce(new Error('offline'))
    wrapper = mount(ShopAvailabilityControl); await flushPromises()
    await wrapper.get('[aria-label="shop.editName: shop.externalEntry"]').trigger('click')
    await wrapper.get('#external-shop-name').setValue('新名称')
    await wrapper.get('#external-shop-name').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(wrapper.find('#external-shop-name').exists()).toBe(true)
    expect(wrapper.findAll('section')[1].get('h2').text()).toBe('shop.externalEntry')
    expect(wrapper.text()).toContain('offline')
    await wrapper.get('#external-shop-name').element.closest('form')!.querySelector('button[type="button"]')!.dispatchEvent(new Event('click', { bubbles: true }))
    await flushPromises()
    expect(wrapper.find('#external-shop-name').exists()).toBe(false)
  })
})
