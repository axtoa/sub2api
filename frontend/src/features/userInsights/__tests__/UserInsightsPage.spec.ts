import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserInsightsPage from '../UserInsightsPage.vue'

const { list, detail } = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn() }))
vi.mock('../api', () => ({ userInsightsAPI: { list, detail } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => key === 'userInsights.pageProgress'
      ? `第 ${params?.page} 页 / 共 ${params?.totalPages} 页`
      : key,
    locale: ref('zh')
  }) }
})

const user = {
  id: 7, email: 'alice@example.com', username: 'Alice', notes: '重点用户', status: 'active',
  balance: '5.00', redeemed_amount: '30.00', redeem_count: 2,
  last_redeemed_at: '2026-10-25T09:00:00Z', usage_30d: '25.00', calls_30d: 3, total_calls: 42,
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
    expect(wrapper.find('tbody').text()).toContain('重点用户')
    expect(list).toHaveBeenCalledWith(1, '', 'all', 'redeemed_amount', 'desc', expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('loads a repeat-redeemer segment and user detail on demand', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('select').setValue('repeat')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'repeat', 'redeemed_amount', 'desc', expect.any(AbortSignal))
    await wrapper.find('button[title="userInsights.viewDetail"]').trigger('click')
    await flushPromises()
    expect(detail).toHaveBeenCalledWith(7, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('gpt-5')
    expect(wrapper.text()).toContain('$4.00')
    wrapper.unmount()
  })

  it('shows recent and lifetime call counts as separate columns', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const headers = wrapper.findAll('thead th').map(header => header.text())
    expect(headers).toContain('userInsights.calls30d')
    expect(headers).toContain('userInsights.totalCalls')
    const cells = wrapper.findAll('tbody tr')[0].findAll('td')
    expect(cells[7].text()).toBe('3')
    expect(cells[8].text()).toBe('42')
    wrapper.unmount()
  })

  it.each([
    ['user', 'email'], ['notes', 'notes'], ['redeemedAmount', 'redeemed_amount'], ['redeemCount', 'redeem_count'],
    ['lastRedeemed', 'last_redeemed_at'], ['balance', 'balance'], ['usage30d', 'usage_30d'],
    ['calls30d', 'calls_30d'], ['totalCalls', 'total_calls'], ['lastUsed', 'last_used_at']
  ])('toggles the %s header in both directions', async (label, key) => {
    const wrapper = mountPage()
    await flushPromises()
    const header = wrapper.findAll('thead th').find(th => th.text() === `userInsights.${label}`)!
    await header.find('button').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'all', key, 'asc', expect.any(AbortSignal))
    expect(header.attributes('aria-sort')).toBe('ascending')
    await header.find('button').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'all', key, 'desc', expect.any(AbortSignal))
    expect(header.attributes('aria-sort')).toBe('descending')
    wrapper.unmount()
  })

  it('resets the page when sorting and keeps sorting across search and pagination', async () => {
    list.mockResolvedValue({
      items: [user], total: 50, page: 1, page_size: 20,
      summary: { amount: '100.00', users: 3, codes: 4 }, filtered_amount: '30.00'
    })
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button[title="userInsights.next"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(2, '', 'all', 'redeemed_amount', 'desc', expect.any(AbortSignal))
    await wrapper.findAll('thead th').find(th => th.text() === 'userInsights.totalCalls')!.find('button').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'all', 'total_calls', 'asc', expect.any(AbortSignal))
    await wrapper.find('input[type="search"]').setValue('alice')
    await wrapper.find('input[type="search"]').trigger('keyup.enter')
    await flushPromises()
    await wrapper.find('button[title="userInsights.next"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(2, 'alice', 'all', 'total_calls', 'asc', expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('supports selecting the sort column and direction on mobile', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('select[aria-label="userInsights.sortBy"]').setValue('calls_30d')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'all', 'calls_30d', 'desc', expect.any(AbortSignal))
    await wrapper.find('button[aria-label="userInsights.sortAscending"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, '', 'all', 'calls_30d', 'asc', expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('shows the total page count on the right side of pagination', async () => {
    list.mockResolvedValue({
      items: [user], total: 41, page: 1, page_size: 20,
      summary: { amount: '100.00', users: 3, codes: 4 }, filtered_amount: '30.00'
    })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('第 1 页 / 共 3 页')
    expect(wrapper.find('.min-w-36').text()).toBe('第 1 页 / 共 3 页')
    await wrapper.find('button[title="userInsights.next"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('.min-w-36').text()).toBe('第 2 页 / 共 3 页')
    expect(list).toHaveBeenLastCalledWith(2, '', 'all', 'redeemed_amount', 'desc', expect.any(AbortSignal))
    wrapper.unmount()
  })
})
