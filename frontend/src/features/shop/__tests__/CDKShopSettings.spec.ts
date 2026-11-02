import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import CDKShopSettings from '../CDKShopSettings.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(), post: vi.fn(), put: vi.fn(), refresh: vi.fn(), fetch: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiClient: mocks }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: mocks.refresh }) }))
vi.mock('../ShopLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('fetch', mocks.fetch)
  mocks.post.mockResolvedValue({ data: { token: 'shop-token' } })
  mocks.get.mockResolvedValue({ data: { enabled: false } })
  mocks.put.mockImplementation(async (_path, data) => ({ data }))
  mocks.fetch.mockImplementation(async (url: string, init: RequestInit) => ({
    ok: true,
    json: async () => url.endsWith('/settings')
      ? init.method === 'PUT' ? JSON.parse(String(init.body)) : { enabled: true }
      : { items: [] },
  }))
})
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals() })

describe('CDK shop controls', () => {
  it('saves the business switch through the isolated shop PUT endpoint', async () => {
    wrapper = mount(CDKShopSettings, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    const business = wrapper.get('[aria-label="商城营业"]')
    expect(business.attributes('aria-checked')).toBe('true')
    await business.trigger('click')
    await flushPromises()
    expect(mocks.fetch).toHaveBeenCalledWith('/api/v1/cdkshop/admin/settings', expect.objectContaining({
      method: 'PUT', body: '{"enabled":false}',
      headers: expect.objectContaining({ Authorization: 'Bearer shop-token' }),
    }))
    expect(business.attributes('aria-checked')).toBe('false')
    expect(mocks.put).not.toHaveBeenCalled()
  })

  it('keeps admin-only visibility independent from business status', async () => {
    wrapper = mount(CDKShopSettings, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    const visibility = wrapper.get('[aria-label="控制台入口仅管理员可见"]')
    expect(visibility.attributes('aria-checked')).toBe('true')
    mocks.fetch.mockClear()
    await visibility.trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledWith('/admin/shop/settings/cdk-menu', { enabled: true })
    expect(mocks.refresh).toHaveBeenCalledWith(true)
    expect(visibility.attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[aria-label="商城营业"]').attributes('aria-checked')).toBe('true')
    expect(mocks.fetch).not.toHaveBeenCalled()
  })

  it('disables unknown settings and preserves independent controls if the shop is offline', async () => {
    mocks.fetch.mockRejectedValue(new Error('商城服务暂不可用'))
    wrapper = mount(CDKShopSettings, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    expect(wrapper.get('[aria-label="商城营业"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="控制台入口仅管理员可见"]').attributes('disabled')).toBeDefined()
    await flushPromises()
    expect(wrapper.get('[aria-label="商城营业"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="控制台入口仅管理员可见"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('商城服务暂不可用')
  })

  it('keeps the confirmed business state after a failed save', async () => {
    wrapper = mount(CDKShopSettings, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    mocks.fetch.mockResolvedValueOnce({ ok: false, json: async () => ({ error: '保存失败' }) })
    await wrapper.get('[aria-label="商城营业"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="商城营业"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[role="alert"]').text()).toContain('保存失败')
  })
})
