import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserInsightsPage from '../UserInsightsPage.vue'

const { list, detail } = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn() }))
vi.mock('../api', () => ({ userInsightsAPI: { list, detail } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }
})

const user = {
  id: 7, email: 'alice@example.com', username: 'Alice', status: 'active',
  balance: '5.00', redeemed_amount: '30.00', redeem_count: 2,
  last_redeemed_at: '2026-10-25T09:00:00Z', usage_30d: '25.00', calls_30d: 3,
  last_used_at: '2026-10-25T09:00:00Z'
}

describe('UserInsightsPage', () => {
  beforeEach(() => {
    list.mockReset()
    detail.mockReset()
    list.mockResolvedValue({
      items: [user], total: 1, page: 1, page_size: 20,
      summary: { amount: '100.00', users: 3, codes: 4 }, filtered_amount: '30.00'
    })
    detail.mockResolvedValue({
      ...user, first_redeemed_at: '2026-10-01T09:00:00Z', admin_added_amount: '4.00',
      redemptions: [{ value: '20.00', used_at: '2026-10-25T09:00:00Z' }],
      models: [{ name: 'gpt-5', calls: 3, cost: '25.00' }]
    })
  })

  const mountPage = () => mount(UserInsightsPage, {
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
      Icon: true,
      RouterLink: { template: '<a><slot /></a>' }
    } }
  })

  it('keeps site-wide and filtered totals distinct', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('$100.00')
    expect(wrapper.text()).toContain('$30.00')
    expect(list).toHaveBeenCalledWith(1, '', 'all', expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('loads a repeat-redeemer segment and user detail on demand', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('select').setValue('repeat')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'repeat', expect.any(AbortSignal))
    await wrapper.find('button[title="userInsights.viewDetail"]').trigger('click')
    await flushPromises()
    expect(detail).toHaveBeenCalledWith(7, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('gpt-5')
    expect(wrapper.text()).toContain('$4.00')
    wrapper.unmount()
  })
})
