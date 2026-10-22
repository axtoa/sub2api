import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ShopAdminPage from '../ShopAdminPage.vue'
import { shopAPI } from '../api'
import { productFixture } from './fixtures'
vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/admin/shop/products' }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', async original => ({ ...await original<typeof import('../api')>(), shopAPI: { products: vi.fn(), save: vi.fn() } }))
const adminPage = () => mount(ShopAdminPage, { global: { stubs: { ShopLayout: { template: '<main><slot /></main>' }, ShopAvailabilityControl: true, Icon: true } } })
describe('shop product editing', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.mocked(shopAPI.products).mockResolvedValue([productFixture()]); vi.mocked(shopAPI.save).mockResolvedValue({ ...productFixture(), version: 4 }) })
  it('saves prices in integer cents and includes the product version', async () => {
    const wrapper = adminPage(); await flushPromises(); await wrapper.get('.shop-product-choice').trigger('click')
    await wrapper.get('input[inputmode="decimal"]').setValue('0.29')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.save).toHaveBeenCalledWith(expect.objectContaining({ id: productFixture().id, version: 3, price_cents: 29 }))
  })
  it('rejects excessive decimals without sending a request', async () => {
    const wrapper = adminPage(); await flushPromises(); await wrapper.get('.shop-product-choice').trigger('click')
    await wrapper.get('input[inputmode="decimal"]').setValue('1.234')
    await wrapper.get('form').trigger('submit'); expect(shopAPI.save).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toBe('shop.invalidPrice')
  })
  it('does not silently overwrite a concurrent product edit', async () => {
    vi.mocked(shopAPI.save).mockRejectedValue({ status: 409, message: 'new version exists' })
    const wrapper = adminPage(); await flushPromises(); await wrapper.get('.shop-product-choice').trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit'); expect(shopAPI.save).toHaveBeenCalledTimes(1)
  })
})
