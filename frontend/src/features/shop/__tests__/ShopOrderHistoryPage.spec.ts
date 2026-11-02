import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import ShopOrderHistoryPage from '../ShopOrderHistoryPage.vue'
import { shopAPI } from '../api'
import { orderFixture } from './fixtures'

enableAutoUnmount(afterEach)
const route = vi.hoisted(() => ({ params: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', async original => ({
  ...await original<typeof import('../api')>(),
  shopAPI: { orders: vi.fn(), order: vi.fn(), products: vi.fn(), create: vi.fn() },
}))
const mountHistory = () => mount(ShopOrderHistoryPage, { global: { stubs: {
  ShopLayout: { template: '<main><slot /></main>' }, RouterLink: RouterLinkStub, Icon: true,
} } })

describe('legacy purchase order history', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.params = {}
    vi.mocked(shopAPI.orders).mockResolvedValue({ items: [orderFixture()], total: 1, page: 1, page_size: 20, pages: 1 })
    vi.mocked(shopAPI.order).mockResolvedValue(orderFixture())
  })
  it('keeps account order history accessible without loading the retired catalogue', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    expect(shopAPI.orders).toHaveBeenCalledWith(false, expect.objectContaining({ page: 1 }))
    expect(wrapper.text()).toContain('SOtest')
    expect(wrapper.text()).toContain('shop.legacyOrdersHint')
    expect(shopAPI.products).not.toHaveBeenCalled()
    expect(shopAPI.create).not.toHaveBeenCalled()
    expect(wrapper.find('input[type="email"]').exists()).toBe(false)
  })
  it('continues to open existing order details and their payment or after-sales actions', async () => {
    route.params = { id: 'SOtest' }
    const wrapper = mountHistory()
    await flushPromises()
    expect(shopAPI.order).toHaveBeenCalledWith('SOtest', false)
    expect(wrapper.text()).toContain('SOtest')
    expect(wrapper.find('form').exists()).toBe(true)
    expect(shopAPI.create).not.toHaveBeenCalled()
  })
})
