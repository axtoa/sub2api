import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub, type VueWrapper } from '@vue/test-utils'
import CDKStorefront from '../CDKStorefront.vue'

const state = vi.hoisted(() => ({
  auth: { isAuthenticated: true, user: { id: 17, email: 'buyer@example.com' } },
  route: { query: {} as Record<string, string> },
  fetch: vi.fn(), post: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ siteName: 'Best AI', siteLogo: '' }) }))
vi.mock('@/api/client', () => ({ apiClient: { post: state.post } }))
vi.mock('vue-router', () => ({ useRoute: () => state.route }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { amount: string }) => key === 'shop.storefrontReward' ? `赠 ${params?.amount} Best AI 余额` : key }) }))
vi.mock('../ShopLayout.vue', () => ({ default: { props: ['embedded'], template: '<div :data-embedded="embedded"><slot /></div>' } }))
vi.mock('qrcode', () => ({ default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,test') } }))

const product = { id: 'product-1', name: 'Plus', spec: '1 个月', price_cents: 9900, reward_cents: 500, stock: 1 }
const order = { id: 'order-1', product_name: 'Plus', price_cents: 9900, payment_status: 'pending', delivery_status: 'pending' }
const response = (body: unknown, ok = true) => ({ ok, json: async () => body })
let wrapper: VueWrapper | undefined
const mountStorefront = () => mount(CDKStorefront, { global: { stubs: { RouterLink: RouterLinkStub } } })

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  state.route.query = {}
  state.auth.isAuthenticated = true
  state.post.mockResolvedValue({ data: { token: 'scoped-shop-token' } })
  vi.stubGlobal('fetch', state.fetch)
  state.fetch.mockImplementation(async (url: string) => {
    if (url.endsWith('/products')) return response({ items: [product] })
    if (url.endsWith('/status')) return response({ enabled: true })
    return response(order)
  })
})
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals() })

describe('CDK storefront order access', () => {
  it('keeps signed-in buyers in the independent storefront', async () => {
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.attributes('data-embedded')).toBe('false')
  })

  it.each([true, false])('shows USD balance rewards with the appropriate purchase conditions when authenticated=%s', async authenticated => {
    state.auth.isAuthenticated = authenticated
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.get('.shop-product .cdk-balance-reward').text()).toBe('赠 $5 Best AI 余额')
    expect(wrapper.get('.cdk-checkout-reward .cdk-balance-reward').text()).toBe('赠 $5 Best AI 余额')
    expect(wrapper.get('.cdk-checkout-reward .shop-caption').text()).toBe(authenticated ? 'shop.storefrontRewardTerms' : 'shop.storefrontRewardGuest')
    expect(wrapper.get('.cdk-checkout-total strong').text()).toBe('¥99.00')
    expect(state.post).not.toHaveBeenCalled()
  })

  it('does not promise balance rewards for a product with no reward configured', async () => {
    state.fetch.mockImplementation(async (url: string) => url.endsWith('/products') ? response({ items: [{ ...product, reward_cents: 0 }] }) : response({ enabled: true }))
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.find('.cdk-balance-reward').exists()).toBe(false)
    expect(wrapper.find('.cdk-checkout-reward').exists()).toBe(false)
  })

  it('preserves the selected product in the reward login link without forcing guests to sign in', async () => {
    state.auth.isAuthenticated = false
    state.fetch.mockImplementation(async (url: string) => url.endsWith('/products')
      ? response({ items: [product, { ...product, id: 'product-2', name: 'Pro' }] }) : response({ enabled: true }))
    wrapper = mountStorefront()
    await flushPromises()
    await wrapper.findAll('.shop-product-choice')[1].trigger('click')
    expect(wrapper.getComponent(RouterLinkStub).props('to')).toEqual({ path: '/shop/login', query: { redirect: '/shop?product=product-2#shop-product-details' } })
    expect(wrapper.get('.cdk-checkout-card .shop-primary').attributes('disabled')).toBeUndefined()
    expect(state.post).not.toHaveBeenCalled()
    expect(wrapper.element.querySelector('.shop-checkout-grid')!.compareDocumentPosition(wrapper.get('#shop-order-lookup').element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('selects the product linked from the homepage rather than the first product', async () => {
    state.route.query = { product: 'product-2' }
    state.fetch.mockImplementation(async (url: string) => url.endsWith('/products')
      ? response({ items: [product, { ...product, id: 'product-2', name: 'Pro', price_cents: 19900 }] })
      : response({ enabled: true }))
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.get('.cdk-checkout-card').text()).toContain('Pro')
    expect(wrapper.get('.cdk-checkout-total').text()).toContain('199.00')
    expect(wrapper.findAll('.shop-product-choice').map(item => item.attributes('aria-pressed'))).toEqual(['false', 'true'])
  })

  it('shows a linked sold-out product without silently selecting another offer', async () => {
    state.route.query = { product: 'product-2' }
    state.fetch.mockImplementation(async (url: string) => url.endsWith('/products')
      ? response({ items: [product, { ...product, id: 'product-2', name: 'Sold out Pro', stock: 0 }] })
      : response({ enabled: true }))
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.get('.cdk-checkout-card').text()).toContain('Sold out Pro')
    expect(wrapper.get('.cdk-checkout-card').text()).toContain('shop.storefrontSoldOutHint')
    expect(wrapper.get('.shop-form button').attributes('disabled')).toBeDefined()
  })

  it('explains when the shared product is no longer available', async () => {
    state.route.query = { product: 'removed-product' }
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.text()).toContain('shop.storefrontProductMissing')
  })

  it('opens order recovery directly from the homepage lookup link', async () => {
    state.auth.isAuthenticated = false
    state.route.query = { lookup: '1' }
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.get('#shop-order-lookup').attributes('open')).toBeDefined()
    expect(state.post).not.toHaveBeenCalled()
  })

  it('uses scoped account authorization to recover an order after returning from payment', async () => {
    state.route.query = { order: order.id }
    wrapper = mountStorefront()
    await flushPromises()
    expect(state.post).toHaveBeenCalledWith('/auth/cdkshop-token')
    expect(state.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/orders/order-1', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer scoped-shop-token' }),
      cache: 'no-store',
    }))
    expect(wrapper.text()).toContain('订单已创建')
  })

  it('lets guests recover an order with a recovery key without issuing an account token', async () => {
    state.auth.isAuthenticated = false
    state.route.query = { order: order.id }
    sessionStorage.setItem('cdkshop-recovery:order-1', 'guest-recovery')
    wrapper = mountStorefront()
    await flushPromises()
    expect(state.post).not.toHaveBeenCalled()
    expect(state.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/orders/order-1', expect.objectContaining({
      headers: expect.objectContaining({ 'X-Order-Recovery-Key': 'guest-recovery' }),
    }))
  })

  it('reuses the request ID after a network failure so retrying does not reserve another code', async () => {
    let attempts = 0
    state.fetch.mockImplementation(async (url: string) => {
      if (url.endsWith('/products')) return response({ items: [product] })
      if (url.endsWith('/status')) return response({ enabled: true })
      attempts++
      if (attempts === 1) throw new Error('network timeout')
      return response({ order, recovery_key: 'saved-key' })
    })
    wrapper = mountStorefront()
    await flushPromises()
    await wrapper.get('.shop-form').trigger('submit')
    await flushPromises()
    await wrapper.get('.shop-form').trigger('submit')
    await flushPromises()
    const calls = state.fetch.mock.calls.filter(([url]) => url === '/api/v1/cdkshop/orders')
    expect(calls).toHaveLength(2)
    expect(JSON.parse(calls[0][1].body).request_id).toBe(JSON.parse(calls[1][1].body).request_id)
    expect(sessionStorage.getItem('cdkshop-recovery:order-1')).toBe('saved-key')
  })

  it('does not render unsafe payment links supplied by the provider', async () => {
    state.route.query = { order: order.id }
    state.fetch.mockResolvedValue(response({ ...order, pay_url: 'javascript:alert(1)' }))
    wrapper = mountStorefront()
    await flushPromises()
    expect(wrapper.find('a[href^="javascript:"]').exists()).toBe(false)
  })
})
