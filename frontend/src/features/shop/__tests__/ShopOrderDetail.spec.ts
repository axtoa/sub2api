import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import ShopOrderDetail from '../ShopOrderDetail.vue'
import { shopAPI } from '../api'
import { orderFixture } from './fixtures'
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'SOtest' } }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../api', async (original) => ({ ...await original<typeof import('../api')>(), shopAPI: { order: vi.fn(), action: vi.fn() } }))
function detail(admin = false) { return mount(ShopOrderDetail, { props: { admin }, global: { stubs: { RouterLink: RouterLinkStub, Icon: true } } }) }
describe('shop order actions', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.mocked(shopAPI.order).mockResolvedValue(orderFixture()); vi.mocked(shopAPI.action).mockResolvedValue({ ...orderFixture(), external_reference: 'receipt', version: 2 }) })
  it('submits a payment reference without confirming payment', async () => {
    const wrapper = detail(); await flushPromises()
    expect(wrapper.get('a.shop-primary').attributes('href')).toBe(orderFixture().product.payment_url)
    await wrapper.get('input[maxlength="200"]').setValue('receipt')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.action).toHaveBeenCalledWith('SOtest', expect.objectContaining({ action: 'payment_reference', version: 1, external_reference: 'receipt' }), false)
    expect(wrapper.findAll('option').map(option => option.attributes('value'))).not.toContain('confirm_paid')
    expect(wrapper.get('[data-status="PENDING"]').exists()).toBe(true)
  })
  it('requires explicit verification for admin payment confirmation', async () => {
    const wrapper = detail(true); await flushPromises()
    await wrapper.get('form').trigger('submit'); expect(shopAPI.action).not.toHaveBeenCalled()
    await wrapper.get('input[maxlength="200"]').setValue('verified-receipt')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.action).toHaveBeenCalledWith('SOtest', expect.objectContaining({ action: 'confirm_paid', version: 1 }), true)
  })
  it('prevents retrying a stale mutation until refreshed', async () => {
    vi.mocked(shopAPI.action).mockRejectedValue({ status: 409, message: 'stale order' })
    const wrapper = detail(); await flushPromises()
    await wrapper.get('input[maxlength="200"]').setValue('receipt')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit'); expect(shopAPI.action).toHaveBeenCalledTimes(1)
  })
  it('does not allow fulfillment or account edits after completion', async () => {
    vi.mocked(shopAPI.order).mockResolvedValue({ ...orderFixture(), payment_status: 'PAID', fulfillment_status: 'COMPLETED' })
    const wrapper = detail(); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('a.shop-primary').exists()).toBe(false)
  })
})
