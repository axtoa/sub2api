import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
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
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../ShopLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('qrcode', () => ({ default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,test') } }))

const product = { id: 'product-1', name: 'Plus', spec: '1 个月', price_cents: 9900, reward_cents: 500, stock: 1 }
const order = { id: 'order-1', product_name: 'Plus', price_cents: 9900, payment_status: 'pending', delivery_status: 'pending' }
const response = (body: unknown, ok = true) => ({ ok, json: async () => body })
let wrapper: VueWrapper | undefined

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
  it('uses scoped account authorization to recover an order after returning from payment', async () => {
    state.route.query = { order: order.id }
    wrapper = mount(CDKStorefront)
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
    wrapper = mount(CDKStorefront)
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
    wrapper = mount(CDKStorefront)
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
    wrapper = mount(CDKStorefront)
    await flushPromises()
    expect(wrapper.find('a[href^="javascript:"]').exists()).toBe(false)
  })
})
