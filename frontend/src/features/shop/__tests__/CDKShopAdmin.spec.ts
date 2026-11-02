import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import CDKShopAdmin from '../CDKShopAdmin.vue'

enableAutoUnmount(afterEach)
const state = vi.hoisted(() => ({ fetch: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: state.post } }))
vi.mock('../ShopLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/common/ConfirmDialog.vue', () => ({ default: {
  props: ['show', 'title', 'message', 'confirmText'], emits: ['confirm', 'cancel'],
  template: '<div v-if="show" role="dialog"><p>{{ message }}</p><button class="confirm-delete" @click="$emit(\'confirm\')">{{ confirmText }}</button><button class="cancel-delete" @click="$emit(\'cancel\')">取消</button></div>',
} }))

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

  it('requires confirmation before deletion and removes the product after success', async () => {
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-delete-product').trigger('click')
    expect(state.fetch.mock.calls.some(([, options]) => options.method === 'DELETE')).toBe(false)
    expect(wrapper.get('[role="dialog"]').text()).toContain('已创建的订单仍可付款和领取兑换码')
    await wrapper.get('.confirm-delete').trigger('click')
    await flushPromises()
    expect(state.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/admin/products/product-1', expect.objectContaining({ method: 'DELETE', headers: expect.objectContaining({ Authorization: 'Bearer scoped-shop-token' }) }))
    expect(wrapper.find('.cdk-admin-item').exists()).toBe(false)
    expect(wrapper.text()).toContain('历史订单与库存记录已保留')
  })

  it('cancels deletion without sending a delete request', async () => {
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-delete-product').trigger('click')
    await wrapper.get('.cancel-delete').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.find('.cdk-admin-item').exists()).toBe(true)
    expect(state.fetch.mock.calls.some(([, options]) => options.method === 'DELETE')).toBe(false)
  })

  it('keeps the product visible and reports a failed deletion', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => options.method === 'DELETE'
      ? Promise.resolve({ ok: false, json: async () => ({ error: 'product_delete_failed' }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-delete-product').trigger('click')
    await wrapper.get('.confirm-delete').trigger('click')
    await flushPromises()
    expect(wrapper.get('.shop-error').text()).toContain('删除失败')
    expect(wrapper.find('.cdk-admin-item').exists()).toBe(true)
  })

  it('takes a product off sale without deleting its inventory', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => options.method === 'PATCH'
      ? Promise.resolve({ ok: true, json: async () => ({ active: false }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.cdk-admin-item-actions .shop-secondary').trigger('click')
    await flushPromises()
    expect(state.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/admin/products/product-1', expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ active: false }) }))
    expect(wrapper.get('.cdk-admin-item').text()).toContain('已下架')
    expect(wrapper.get('.cdk-admin-item-actions .shop-secondary').text()).toBe('重新上架')
    expect(wrapper.find('.cdk-import').exists()).toBe(true)
  })
})
