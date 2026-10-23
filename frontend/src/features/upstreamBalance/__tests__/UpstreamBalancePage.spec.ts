import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import Page from '../UpstreamBalancePage.vue'
import { balanceAPI, type Wallet } from '../api'
import { actualTotals, money, safeURL } from '../display'
import en from '@/i18n/locales/en/upstreamBalance'

function lookup(key: string) { return key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, en) }
vi.mock('vue-i18n', () => ({ useI18n: () => ({ te: (key: string) => typeof lookup(key) === 'string', t: (key: string, params: Record<string, unknown> = {}) => {
  const value = lookup(key); return typeof value === 'string' ? value.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`)) : key
} }) }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: defineComponent({ template: '<main><slot /></main>' }) }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: defineComponent({ props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' }) }))
const account = { id: 1, name: 'Local account', platform: 'anthropic', type: 'apikey', status: 'active', wallet_id: 'w1' }
const wallet: Wallet = { id: 'w1', name: 'Provider A', provider: 'Main wallet', mode: 'sub2api', query_account_id: 1, account_ids: [1], currency: 'USD', auto_enabled: false, interval_minutes: 30, estimate_enabled: false, low_threshold: '10', management_url: 'https://example.com', note: '', version: 2, observation: { amount: '100.123456', currency: 'USD', scope: 'wallet', at: '2026-10-23T02:00:00Z' }, status: 'ok', last_error: '', last_attempt_at: '2026-10-23T02:00:00Z', next_query_at: '2026-10-23T02:30:00Z', accounts: [account], today_cost: '3.000001', week_cost: '70', month_cost: '75', since_baseline_cost: '0', estimated_balance: null, days_remaining: '10.0', stale: false, low: false, usage_incomplete: false }
const settings = { settings: { email_enabled: false, recipients: [], version: 1 }, deliveries: [], timezone: 'Asia/Shanghai' }
const wrappers: Array<ReturnType<typeof mount>> = []
function render() { const wrapper = mount(Page); wrappers.push(wrapper); return wrapper }
function button(wrapper: ReturnType<typeof mount>, text: string) { const found = wrapper.findAll('button').find(b => b.text() === text); expect(found, text).toBeDefined(); return found! }
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-23T02:00:00Z'))
  vi.spyOn(balanceAPI, 'list').mockResolvedValue({ items: [structuredClone(wallet)], timezone: 'Asia/Shanghai', server_time: '2026-10-23T02:00:00Z' })
  vi.spyOn(balanceAPI, 'accounts').mockResolvedValue([structuredClone(account)])
  vi.spyOn(balanceAPI, 'save').mockResolvedValue(wallet); vi.spyOn(balanceAPI, 'calibrate').mockResolvedValue(wallet); vi.spyOn(balanceAPI, 'query').mockResolvedValue(wallet)
  vi.spyOn(balanceAPI, 'settings').mockResolvedValue(structuredClone(settings)); vi.spyOn(balanceAPI, 'saveSettings').mockResolvedValue(settings.settings); vi.spyOn(balanceAPI, 'history').mockResolvedValue([])
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.restoreAllMocks(); vi.useRealTimers() })

describe('Upstream wallet balances', () => {
  it('adds exact actual wallet funds only, never quotas, plans, manual or stale amounts', () => {
    const rows = [wallet, { ...wallet, id: 'w2', observation: { ...wallet.observation!, amount: '0.000001' } }, { ...wallet, stale: true }, { ...wallet, status: 'failed' }, { ...wallet, mode: 'manual' as const }, { ...wallet, observation: { ...wallet.observation!, scope: 'key_quota' as const } }, { ...wallet, observation: { ...wallet.observation!, scope: 'subscription' as const } }]
    expect(actualTotals(rows)).toEqual([{ currency: 'USD', amount: '100.123457' }]); expect(money('-0.000001')).toBe('-0.000001'); expect(safeURL('javascript:alert(1)')).toBe(false)
    expect(actualTotals([{ ...wallet, observation: { ...wallet.observation!, amount: '-0.000001' } }])[0]!.amount).toBe('-0.000001')
  })
  it('renders a wallet card, current cost snapshots and no automatic upstream request on page open', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('.balance-card')).toHaveLength(1); expect(wrapper.text()).toContain('100.123456'); expect(wrapper.text()).toContain('Local account'); expect(wrapper.text()).toContain('3.000001')
    expect(balanceAPI.query).not.toHaveBeenCalled(); expect(balanceAPI.save).not.toHaveBeenCalled()
  })
  it('manual query works repeatedly even with scheduling disabled', async () => {
    const wrapper = render(); await flushPromises()
    for (let i = 0; i < 2; i++) { await button(wrapper, en.upstreamBalance.query).trigger('click'); await flushPromises() }
    expect(balanceAPI.query).toHaveBeenCalledTimes(2); expect(balanceAPI.query).toHaveBeenCalledWith('w1')
  })
  it('keeps the last successful value visibly stale on failures', async () => {
    vi.mocked(balanceAPI.list).mockResolvedValue({ items: [{ ...wallet, stale: true, status: 'failed', last_error: 'request_failed', days_remaining: null }], timezone: 'Asia/Shanghai', server_time: '2026-10-23T02:00:00Z' })
    const wrapper = render(); await flushPromises(); expect(wrapper.text()).toContain('100.123456'); expect(wrapper.text()).toContain(en.upstreamBalance.lastKnown)
    expect(wrapper.get('.balance-totals').text()).toBe('—'); expect(wrapper.text()).not.toContain('About 10.0 days')
  })
  it('retries uncertain writes with the same key and preserves the selected query source', async () => {
    vi.mocked(balanceAPI.save).mockRejectedValueOnce({ status: 500 })
    const wrapper = render(); await flushPromises(); await button(wrapper, en.upstreamBalance.edit).trigger('click'); await flushPromises()
    await wrapper.get('#upstream-wallet-form').trigger('submit'); await flushPromises(); const first = vi.mocked(balanceAPI.save).mock.calls[0]!
    expect(first[0]).toBe('w1'); expect(first[1]).toMatchObject({ query_account_id: 1, account_ids: [1], version: 2, initial_balance: null })
    await wrapper.get('#upstream-wallet-form').trigger('submit'); await flushPromises(); expect(vi.mocked(balanceAPI.save).mock.calls[1]![2]).toBe(first[2])
  })
  it('never sends secrets in the wallet form or displays upstream errors verbatim', async () => {
    vi.mocked(balanceAPI.query).mockRejectedValue(new Error('secret-api-key'))
    const wrapper = render(); await flushPromises(); await button(wrapper, en.upstreamBalance.query).trigger('click'); await flushPromises()
    expect(wrapper.text()).not.toContain('secret-api-key'); await button(wrapper, en.upstreamBalance.edit).trigger('click'); await flushPromises()
    expect(wrapper.find('input[type="password"]').exists()).toBe(false); expect(wrapper.find('input[name="api_key"]').exists()).toBe(false)
  })
  it('filters cards without refetching or probing upstreams', async () => {
    const wrapper = render(); await flushPromises(); await wrapper.get('.balance-filters input').setValue('missing'); expect(wrapper.findAll('.balance-card')).toHaveLength(0)
    await wrapper.get('.balance-filters input').setValue('Local account'); expect(wrapper.findAll('.balance-card')).toHaveLength(1); expect(balanceAPI.list).toHaveBeenCalledTimes(1); expect(balanceAPI.query).not.toHaveBeenCalled()
  })
  it('blocks account forms when options cannot be loaded', async () => {
    vi.mocked(balanceAPI.accounts).mockRejectedValue(new Error('database password'))
    const wrapper = render(); await flushPromises(); await button(wrapper, en.upstreamBalance.edit).trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toBe(en.upstreamBalance.loadError); expect(button(wrapper, en.upstreamBalance.save).attributes('disabled')).toBeDefined()
  })
  it('leaves low-balance email disabled until explicitly saved', async () => {
    const wrapper = render(); await flushPromises(); await button(wrapper, en.upstreamBalance.settings).trigger('click'); await flushPromises()
    expect((wrapper.get('#upstream-settings-form input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('#upstream-settings-form').trigger('submit'); await flushPromises(); expect(balanceAPI.saveSettings).toHaveBeenCalledWith(settings.settings)
  })
  it('polls snapshots only and clears timers and request signals on unmount', async () => {
    const wrapper = render(); await flushPromises(); await vi.advanceTimersByTimeAsync(60000); await flushPromises(); expect(balanceAPI.list).toHaveBeenCalledTimes(2); expect(balanceAPI.query).not.toHaveBeenCalled()
    const signal = vi.mocked(balanceAPI.list).mock.calls.at(-1)![0]!; wrapper.unmount(); expect(signal.aborted).toBe(true); await vi.advanceTimersByTimeAsync(60000); expect(balanceAPI.list).toHaveBeenCalledTimes(2)
  })
})
