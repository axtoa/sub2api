import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import HomeShopSection from '../HomeShopSection.vue'

enableAutoUnmount(afterEach)
const state = vi.hoisted(() => ({
  app: { cachedPublicSettings: {} as Record<string, unknown> },
  fetch: vi.fn(),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => state.app }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { amount: string }) => key === 'shop.storefrontReward' ? `赠 ${params?.amount} Best AI 余额` : key }) }))

const response = (body: unknown, ok = true) => ({ ok, json: async () => body })
const products = [
  { id: 'product-1', name: 'Plus', spec: '1 month', price_cents: 9900, stock: 2 },
  { id: 'product-2', name: 'Pro', spec: '1 month', price_cents: 19900, stock: 0 },
]

beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('fetch', state.fetch)
  state.app.cachedPublicSettings = {}
  state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: products } : { enabled: true }))
})
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs() })

describe('homepage storefront entry', () => {
  it('shows real prices and links directly to each product in a new tab', async () => {
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    const cards = wrapper.findAll('.home-shop-product')
    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('¥99.00')
    expect(cards[0].attributes('href')).toBe('/shop?product=product-1')
    expect(cards[1].text()).toContain('shop.storefrontSoldOut')
    for (const link of wrapper.findAll('a')) {
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toBe('noopener noreferrer')
      expect(link.attributes('href')).not.toMatch(/login|options/)
    }
    expect(wrapper.get('.home-shop-cta').attributes('href')).toBe('/shop')
    expect(wrapper.get('a[href*="lookup=1"]').attributes('href')).toBe('/shop?lookup=1#shop-order-lookup')
  })

  it('honors a dedicated storefront URL for product, browse, and lookup links', async () => {
    vi.stubEnv('VITE_STOREFRONT_URL', 'https://shop.example.com/shop')
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    expect(wrapper.get('.home-shop-product').attributes('href')).toBe('https://shop.example.com/shop?product=product-1')
    expect(wrapper.get('.home-shop-cta').attributes('href')).toBe('https://shop.example.com/shop')
    expect(wrapper.get('a[href*="lookup=1"]').attributes('href')).toBe('https://shop.example.com/shop?lookup=1#shop-order-lookup')
  })

  it.each([[500, '$5'], [525, '$5.25']])('shows a USD balance reward of %s cents separately from the CNY price', async (cents, amount) => {
    state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: [{ ...products[0], reward_cents: cents }, { ...products[1], reward_cents: 0 }] } : { enabled: true }))
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    expect(wrapper.findAll('.home-shop-reward')).toHaveLength(1)
    expect(wrapper.get('.home-shop-reward').text()).toBe(`赠 ${amount} Best AI 余额`)
    expect(wrapper.get('.home-shop-product-end strong').text()).toBe('¥99.00')
    expect(wrapper.get('#home-shop-reward-terms').text()).toBe('shop.storefrontRewardTerms')
    expect(wrapper.get('.home-shop-product').attributes('aria-label')).toContain(`赠 ${amount} Best AI 余额`)
    expect(wrapper.get('.home-shop-product').attributes('aria-describedby')).toBe('home-shop-reward-terms')
  })

  it('does not advertise old configured rewards or rewards belonging to products outside the first two', async () => {
    state.app.cachedPublicSettings = { shop_homepage_content: { title: 'Shop', lead: 'Explore', cards: [{ bonus_cents: 500 }] } }
    state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: [...products, { ...products[0], id: 'product-3', reward_cents: 500 }] } : { enabled: true }))
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    expect(wrapper.find('.home-shop-reward').exists()).toBe(false)
    expect(wrapper.find('#home-shop-reward-terms').exists()).toBe(false)
  })

  it('keeps existing external-only channels public without fetching the internal catalog', async () => {
    state.app.cachedPublicSettings = { shop_homepage_content: { title: 'Our shop', lead: 'Explore', cards: [{ title: 'Subscription', summary: 'Monthly plans', price_hint: 'old price', bonus_cents: 100 }] } }
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: false, externalUrl: 'https://external.example/shop' } })
    await flushPromises()
    expect(state.fetch).not.toHaveBeenCalled()
    expect(wrapper.get('.home-shop-product').attributes('href')).toBe('https://external.example/shop')
    expect(wrapper.get('.home-shop-cta').attributes('href')).toBe('https://external.example/shop')
    expect(wrapper.text()).toContain('Our shop')
    expect(wrapper.text()).not.toContain('Subscription')
    expect(wrapper.text()).not.toContain('old price')
    expect(wrapper.find('a[href*="lookup"]').exists()).toBe(false)
  })

  it('shows only the first two real products in catalog order, ignoring old promotion cards', async () => {
    state.app.cachedPublicSettings = { shop_homepage_content: { title: 'Shop', lead: 'Explore', cards: [{ title: 'Obsolete offer', summary: 'Old promotion', price_hint: '¥1', bonus_cents: 999 }] } }
    state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: [...products, { ...products[0], id: 'product-3', name: 'Third product' }] } : { enabled: true }))
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: 'https://external.example/shop' } })
    await flushPromises()
    expect(wrapper.findAll('.home-shop-product').map(card => card.attributes('href'))).toEqual(['/shop?product=product-1', '/shop?product=product-2'])
    expect(wrapper.text()).not.toContain('Obsolete offer')
    expect(wrapper.text()).not.toContain('Third product')
    expect(wrapper.get('.home-shop-alternative').attributes('href')).toBe('https://external.example/shop')
  })

  it('does not display stale promotional prices when the real catalog is unavailable, and can retry', async () => {
    state.app.cachedPublicSettings = { shop_homepage_content: { title: 'Shop', lead: 'Explore', cards: [{ title: 'Old offer', price_hint: '¥1' }] } }
    state.fetch.mockResolvedValue(response({}, false))
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    expect(wrapper.text()).toContain('shop.loadError')
    expect(wrapper.find('.home-shop-product').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('¥1')
    state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: products } : { enabled: true }))
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.home-shop-product')).toHaveLength(2)
  })

  it('keeps order lookup available when sales are paused', async () => {
    state.fetch.mockImplementation(async (url: string) => response(url.endsWith('/products') ? { items: products } : { enabled: false }))
    const wrapper = mount(HomeShopSection, { props: { officialEnabled: true, externalUrl: '' } })
    await flushPromises()
    expect(wrapper.text()).toContain('shop.storefrontPaused')
    expect(wrapper.find('.home-shop-product').exists()).toBe(false)
    expect(wrapper.find('a[href*="lookup=1"]').exists()).toBe(true)
  })
})
