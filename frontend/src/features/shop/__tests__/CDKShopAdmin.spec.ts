import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import CDKShopAdmin from '../CDKShopAdmin.vue'

enableAutoUnmount(afterEach)
const state = vi.hoisted(() => ({ fetch: vi.fn(), post: vi.fn(), copy: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: state.post } }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: state.copy }) }))
vi.mock('../ShopLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: {
  props: ['show', 'title'], emits: ['close'],
  template: '<div v-if="show" role="dialog"><h3>{{ title }}</h3><button class="close-dialog" @click="$emit(\'close\')">关闭</button><slot /></div>',
} }))
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
      : [{ id: 'product-1', name: 'Plus', spec: '1 month', description: 'Original description', instructions: 'Original instructions', price_cents: 9900, reward_cents: 500, stock: 1, active: true, version: 3 }] }),
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
      ? Promise.resolve({ ok: true, json: async () => ({ active: false, version: 4 }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.cdk-admin-item-actions .shop-secondary').trigger('click')
    await flushPromises()
    expect(state.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/admin/products/product-1', expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ active: false }) }))
    expect(wrapper.get('.cdk-admin-item').text()).toContain('已下架')
    expect(wrapper.get('.cdk-admin-item-actions .shop-secondary').text()).toBe('重新上架')
    expect(wrapper.find('.cdk-import').exists()).toBe(true)
    await wrapper.get('.shop-edit-product').trigger('click')
    await wrapper.get('.cdk-product-edit').trigger('submit')
    await flushPromises()
    const update = state.fetch.mock.calls.find(([, options]) => options.method === 'PUT')
    expect(JSON.parse(update![1].body)).toMatchObject({ version: 4, active: false })
  })

  it('prefills and edits all product details, then refreshes the saved list', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => options.method === 'PUT'
      ? Promise.resolve({ ok: true, json: async () => ({ version: 4 }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-edit-product').trigger('click')
    const form = wrapper.get('.cdk-product-edit')
    const inputs = form.findAll('input')
    expect((inputs[0].element as HTMLInputElement).value).toBe('Plus')
    expect((inputs[2].element as HTMLInputElement).value).toBe('99.00')
    expect((inputs[3].element as HTMLInputElement).value).toBe('5.00')
    const textareas = form.findAll('textarea')
    expect((textareas[0].element as HTMLTextAreaElement).value).toBe('Original description')
    expect((textareas[1].element as HTMLTextAreaElement).value).toBe('Original instructions')
    await inputs[0].setValue('Plus updated')
    await inputs[1].setValue('year')
    await inputs[2].setValue('123.45')
    await inputs[3].setValue('6.25')
    await inputs[4].setValue(false)
    await textareas[0].setValue('New description')
    await textareas[1].setValue('New instructions')
    await form.trigger('submit')
    await flushPromises()
    const update = state.fetch.mock.calls.find(([, options]) => options.method === 'PUT')
    expect(update![0]).toBe('/api/v1/cdkshop/admin/products/product-1')
    expect(JSON.parse(update![1].body)).toEqual({ name: 'Plus updated', spec: 'year', description: 'New description', instructions: 'New instructions', active: false, price_cents: 12345, reward_cents: 625, version: 3 })
    expect(wrapper.find('.cdk-product-edit').exists()).toBe(false)
    expect(wrapper.text()).toContain('商品修改已保存')
    expect(state.fetch.mock.calls.filter(([url, options]) => url.endsWith('/admin/products') && options.method === 'GET')).toHaveLength(2)
  })

  it('cancels an edit without saving and keeps the original product intact', async () => {
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-edit-product').trigger('click')
    await wrapper.get('.cdk-product-edit input').setValue('Unsaved')
    await wrapper.get('.close-dialog').trigger('click')
    expect(wrapper.get('.cdk-admin-item-heading strong').text()).toBe('Plus')
    expect(state.fetch.mock.calls.some(([, options]) => options.method === 'PUT')).toBe(false)
  })

  it('retains the draft after a conflict and does not replay the save', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => options.method === 'PUT'
      ? Promise.resolve({ ok: false, json: async () => ({ error: 'product_conflict' }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-edit-product').trigger('click')
    await wrapper.get('.cdk-product-edit input').setValue('My draft')
    await wrapper.get('.cdk-product-edit').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.cdk-product-edit [role="alert"]').text()).toContain('商品已被其他操作修改')
    expect((wrapper.get('.cdk-product-edit input').element as HTMLInputElement).value).toBe('My draft')
    expect(state.fetch.mock.calls.filter(([, options]) => options.method === 'PUT')).toHaveLength(1)
  })

  it('loads full codes only when requested, supports copying, status filters and pagination, and clears them on close', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => url.includes('/cdks?')
      ? Promise.resolve({ ok: true, json: async () => ({ items: [{ id: 1, code: 'FULL-ADMIN-ONLY-CDK', status: 'reserved', order_id: 'existing-order', created_at: '2026-11-03T00:00:00Z' }], total: 51, page_size: 50 }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    expect(state.fetch.mock.calls.some(([url]) => url.includes('/cdks?'))).toBe(false)
    await wrapper.get('.shop-view-inventory').trigger('click')
    await flushPromises()
    expect(wrapper.get('.cdk-inventory-code code').text()).toBe('FULL-ADMIN-ONLY-CDK')
    expect(wrapper.get('.cdk-inventory-order').text()).toContain('existing-order')
    await wrapper.get('.shop-copy-button').trigger('click')
    expect(state.copy).toHaveBeenCalledWith('FULL-ADMIN-ONLY-CDK', expect.any(String))
    await wrapper.get('.cdk-inventory-pagination button:last-child').trigger('click')
    await flushPromises()
    expect(state.fetch.mock.calls.at(-1)![0]).toContain('page=2')
    await wrapper.get('.cdk-inventory select').setValue('available')
    await flushPromises()
    expect(state.fetch.mock.calls.at(-1)![0]).toContain('page=1&status=available')
    await wrapper.get('.close-dialog').trigger('click')
    expect(wrapper.text()).not.toContain('FULL-ADMIN-ONLY-CDK')
  })

  it('keeps readable codes visible when another record cannot be decrypted', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => url.includes('/cdks?')
      ? Promise.resolve({ ok: true, json: async () => ({ items: [
        { id: 1, code: '', code_error: 'inventory_decrypt_failed', status: 'available', order_id: '', created_at: '2026-11-03T00:00:00Z' },
        { id: 2, code: 'READABLE-CODE', status: 'available', order_id: '', created_at: '2026-11-03T00:00:00Z' },
      ], total: 2, page_size: 50 }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-view-inventory').trigger('click')
    await flushPromises()
    expect(wrapper.get('.cdk-inventory [role="alert"]').text()).toContain('此条兑换码无法解密')
    expect(wrapper.get('.cdk-inventory-code code').text()).toBe('READABLE-CODE')
    expect(wrapper.findAll('.shop-copy-button')).toHaveLength(1)
  })

  it('shows an inventory error without displaying old codes', async () => {
    const originalFetch = state.fetch.getMockImplementation()!
    state.fetch.mockImplementation((url, options) => url.includes('/cdks?')
      ? Promise.resolve({ ok: false, json: async () => ({ error: 'inventory_unavailable' }) })
      : originalFetch(url, options))
    const wrapper = mount(CDKShopAdmin)
    await flushPromises()
    await wrapper.get('.shop-view-inventory').trigger('click')
    await flushPromises()
    expect(wrapper.get('.cdk-inventory [role="alert"]').text()).toContain('库存加载失败')
    expect(wrapper.find('.cdk-inventory-code').exists()).toBe(false)
  })
})
