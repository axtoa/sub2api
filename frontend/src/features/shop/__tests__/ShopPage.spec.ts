import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { reactive } from 'vue'
import ShopPage from '../ShopPage.vue'
import HomeShopSection from '../HomeShopSection.vue'
import { shopAPI } from '../api'
import { orderFixture, productFixture } from './fixtures'
enableAutoUnmount(afterEach)
const { auth, push, route, appState } = vi.hoisted(() => ({
  auth: { user: { email: 'member@example.com' }, isAuthenticated: true },
  push: vi.fn(), route: { path: '/shop', fullPath: '/shop', query: {} as Record<string, string>, params: {} },
  appState: { cachedPublicSettings: { official_shop_enabled: true }, fetchPublicSettings: vi.fn() },
}))
const app = reactive(appState)
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', async (original) => ({ ...await original<typeof import('../api')>(), shopAPI: { products: vi.fn(), create: vi.fn() } }))
const mountShop = () => mount(ShopPage, { global: { stubs: { ShopLayout: { template: '<main><slot /></main>' }, ShopOrders: true, Icon: true, RouterLink: RouterLinkStub } } })
describe('shop purchase form', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.clearAllMocks(); auth.isAuthenticated = true; route.path = '/shop'; route.query = {}; app.cachedPublicSettings.official_shop_enabled = true
    Object.assign(app.cachedPublicSettings, { shop_homepage_content: null, shop_homepage_enabled: true })
    vi.mocked(shopAPI.products).mockResolvedValue([productFixture()])
    vi.mocked(shopAPI.create).mockResolvedValue(orderFixture())
  })
  it('defaults to the login email and never supplies applicant identity or an amount', async () => {
    const wrapper = mountShop(); await flushPromises()
    expect((wrapper.get('input[type="email"]').element as HTMLInputElement).value).toBe('member@example.com')
    expect(wrapper.findAll('input[type="password"]')).toHaveLength(0)
    expect(wrapper.findAll('input[name="session"]')).toHaveLength(0)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('input[type="email"]').setValue('')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.create).toHaveBeenCalledWith({ product_id: productFixture().id, product_version: 3, target_email: '', note: '', accepted: true, request_key: expect.any(String) })
    expect(push).toHaveBeenCalledWith('/shop/orders/SOtest')
  })
  it('does not submit without consent and resets consent after account changes', async () => {
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(shopAPI.create).not.toHaveBeenCalled()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('input[type="email"]').setValue('other@example.com')
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  })
  it('reuses its idempotency key after a network failure', async () => {
    vi.mocked(shopAPI.create).mockRejectedValueOnce({ status: 0, message: 'offline' })
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const calls = vi.mocked(shopAPI.create).mock.calls
    expect(calls).toHaveLength(2); expect(calls[0][0].request_key).toBe(calls[1][0].request_key)
  })
  it('can submit without the secure-context-only randomUUID API', async () => {
    vi.spyOn(crypto, 'randomUUID').mockImplementation(() => { throw new Error('HTTP origin') })
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.create).toHaveBeenCalledWith(expect.objectContaining({
      request_key: expect.stringMatching(/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/),
    }))
    expect(push).toHaveBeenCalledWith('/shop/orders/SOtest')
  })
  it('recovers the submit button if secure randomness is unavailable', async () => {
    const random = vi.spyOn(crypto, 'getRandomValues').mockImplementation(() => { throw new Error('unavailable') })
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.create).not.toHaveBeenCalled()
    expect(wrapper.get('.shop-error').text()).toBe('shop.requestIdError')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    random.mockRestore()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.create).toHaveBeenCalledOnce()
  })
  it('requires refresh and new consent after a price/version conflict', async () => {
    vi.mocked(shopAPI.create).mockRejectedValue({ status: 409, message: 'price changed' })
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.create).toHaveBeenCalledTimes(1)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('.shop-error button').trigger('click'); await flushPromises()
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  })
  it('does not offer checkout for an unconfigured product', async () => {
    vi.mocked(shopAPI.products).mockResolvedValue([{ ...productFixture(), payment_url: '' }])
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.get('button[type="submit"]').text()).toBe('shop.upcoming')
    await wrapper.get('form').trigger('submit'); expect(shopAPI.create).not.toHaveBeenCalled()
  })
  it('links anonymous visitors to login instead of submitting', async () => {
    auth.isAuthenticated = false
    const wrapper = mountShop(); await flushPromises()
    expect(wrapper.find('button[type="submit"]').exists()).toBe(false)
    expect(wrapper.findAllComponents(RouterLinkStub).some(link => (link.props('to') as {path?: string}).path === '/login')).toBe(true)
  })
  it('does not load or offer checkout while purchasing is disabled', async () => {
    app.cachedPublicSettings.official_shop_enabled = false
    const wrapper = mountShop(); await flushPromises()
    expect(wrapper.text()).toContain('shop.paused')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(shopAPI.products).not.toHaveBeenCalled()
    expect(shopAPI.create).not.toHaveBeenCalled()
  })
  it('loads the catalogue when its URL has a trailing slash', async () => {
    route.path = '/shop/'
    const wrapper = mountShop(); await flushPromises()
    expect(shopAPI.products).toHaveBeenCalledOnce()
    expect(wrapper.find('form').exists()).toBe(true)
  })
  it('keeps existing orders accessible while disabled', async () => {
    app.cachedPublicSettings.official_shop_enabled = false
    route.path = '/shop/orders'
    const wrapper = mountShop(); await flushPromises()
    expect(wrapper.findComponent({ name: 'ShopOrders' }).exists()).toBe(true)
    expect(wrapper.text()).not.toContain('shop.paused')
    expect(shopAPI.products).not.toHaveBeenCalled()
  })
  it('hides a stale checkout when the server closes new orders', async () => {
    vi.mocked(shopAPI.create).mockRejectedValue({ status: 403, reason: 'SHOP_DISABLED', message: 'closed' })
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(app.cachedPublicSettings.official_shop_enabled).toBe(false)
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('shop.paused')
  })
  it('does not restore an in-flight catalogue after the switch closes', async () => {
    let resolve!: (value: ReturnType<typeof productFixture>[]) => void
    vi.mocked(shopAPI.products).mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mountShop(); await flushPromises()
    app.cachedPublicSettings.official_shop_enabled = false
    await flushPromises()
    resolve([productFixture()]); await flushPromises()
    expect(wrapper.text()).toContain('shop.paused')
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })
  it('requires a fresh catalogue and consent after reopening', async () => {
    const wrapper = mountShop(); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    app.cachedPublicSettings.official_shop_enabled = false; await flushPromises()
    app.cachedPublicSettings.official_shop_enabled = true; await flushPromises()
    expect(shopAPI.products).toHaveBeenCalledTimes(2)
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    wrapper.unmount()
  })
  it('shows independent homepage cards and opens the channel selector in the same tab', async () => {
    Object.assign(app.cachedPublicSettings, { shop_homepage_content: {
      title: '官方订阅', lead: '选择合适的方案', cards: [{ title: 'Plus', summary: '月度方案', price_hint: '约 ¥200 / 月', bonus_cents: 500 }],
    } })
    const wrapper = mount(HomeShopSection, { global: { stubs: { Icon: true, RouterLink: RouterLinkStub } } }); await flushPromises()
    expect(wrapper.text()).toContain('官方订阅')
    expect(wrapper.text()).toContain('Plus')
    expect(wrapper.text()).toContain('shop.homepageBonusLabel')
    expect(wrapper.text()).toContain('shop.homepageManualBonus')
    expect(wrapper.get('.home-shop-cta').findComponent(RouterLinkStub).props('to')).toBe('/shop/options')
    expect(wrapper.get('.home-shop-cta').attributes('target')).toBeUndefined()
    expect(shopAPI.products).not.toHaveBeenCalled()
  })
  it('does not promise a bonus when none is configured', async () => {
    const wrapper = mount(HomeShopSection, { global: { stubs: { Icon: true, RouterLink: RouterLinkStub } } }); await flushPromises()
    expect(wrapper.text()).not.toContain('shop.homepageManualBonus')
  })
})
