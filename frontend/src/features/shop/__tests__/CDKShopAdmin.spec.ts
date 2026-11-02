import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import CDKShopAdmin from '../CDKShopAdmin.vue'

enableAutoUnmount(afterEach)
const state = vi.hoisted(() => ({ fetch: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: state.post } }))
vi.mock('../ShopLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('fetch', state.fetch)
  state.post.mockResolvedValue({ data: { token: 'scoped-shop-token' } })
  state.fetch.mockImplementation(async (url: string) => ({
    ok: true,
    json: async () => ({ items: url.endsWith('/admin/rewards')
      ? [{ order_id: 'order-1', user_id: 17, email: 'buyer@example.com', amount_cents: 500, status: 'pending' }]
      : [{ id: 'product-1', name: 'Plus', spec: '1 month', price_cents: 9900, reward_cents: 500, stock: 1, active: true }] }),
  }))
})
afterEach(() => vi.unstubAllGlobals())

describe('CDK product reward currency', () => {
  it('keeps CNY prices and USD rewards distinct when showing products, entering rewards, and reviewing tasks', async () => {
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    expect(wrapper.get('.cdk-admin-product').text()).toContain('赠送 Best AI 余额（美元）')
    expect(wrapper.get('.cdk-admin-item').text()).toContain('¥99.00')
    expect(wrapper.get('.cdk-admin-item').text()).toContain('赠 $5 Best AI 余额')
    expect(wrapper.get('.cdk-reward-row strong').text()).toContain('$5 Best AI 余额')
    const inputs = wrapper.findAll('.cdk-admin-product input')
    await inputs[0].setValue('New product')
    await inputs[2].setValue('99')
    await inputs[3].setValue('5.25')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const createCall = state.fetch.mock.calls.find(([url, options]) => url.endsWith('/admin/products') && options.method === 'POST')
    expect(JSON.parse(createCall![1].body)).toMatchObject({ price_cents: 9900, reward_cents: 525 })
  })
})
