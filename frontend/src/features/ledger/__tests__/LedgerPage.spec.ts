import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import LedgerPage from '../LedgerPage.vue'
import { ledgerAPI, type Entry, type Resource } from '../api'
import { calendarDays, localDatetime, isoDatetime, money } from '../display'
import en from '@/i18n/locales/en/ledger'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => {
  const value = key.split('.').reduce<unknown>((v, p) => v && typeof v === 'object' ? (v as Record<string, unknown>)[p] : undefined, en)
  return typeof value === 'string' ? value.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`)) : key
} }) }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: defineComponent({ template: '<main><slot /></main>' }) }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: defineComponent({ props: ['show','title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' }) }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: defineComponent({ template: '<span />' }) }))
const entry: Entry = { id: 'e1', kind: 'expense', amount: '12.500001', currency: 'USD', category: 'Server', occurred_on: '2026-10-23', counterparty: 'Provider', note: 'Example', resource_id: 'r1', voided: false, version: 2, created_at: '2026-10-23T00:00:00Z' }
const resource: Resource = { id: 'r1', name: 'example.com', kind: 'domain', provider: 'Provider', expires_at: '2026-10-30T12:00:00Z', renewal_amount: '20.000000', currency: 'USD', management_url: 'https://example.com/manage', note: '', active: true, version: 3 }
const cfg = { settings: { email_enabled: false, recipients: [], reminder_days: [30,7,1], version: 4 }, timezone: 'Asia/Shanghai', deliveries: [] }
const result = { items: [entry], total: 1, totals: [{ currency: 'USD', income: '100.000000', expense: '12.500001', gift: '5.000000', net: '87.499999' }], categories: ['Server'], today: '2026-10-23', page_size: 30 }
const wrappers: Array<ReturnType<typeof mount>> = []
function render() { const wrapper = mount(LedgerPage); wrappers.push(wrapper); return wrapper }
function button(wrapper: ReturnType<typeof mount>, text: string) { const match = wrapper.findAll('button').find(b => b.text() === text); expect(match, text).toBeDefined(); return match! }
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-23T02:00:00Z'))
  vi.spyOn(ledgerAPI,'entries').mockResolvedValue(structuredClone(result)); vi.spyOn(ledgerAPI,'resources').mockResolvedValue({ items: [structuredClone(resource)], timezone: 'Asia/Shanghai', server_time: '2026-10-23T02:00:00Z' })
  vi.spyOn(ledgerAPI,'settings').mockResolvedValue(structuredClone(cfg)); vi.spyOn(ledgerAPI,'saveEntry').mockResolvedValue(entry); vi.spyOn(ledgerAPI,'saveResource').mockResolvedValue(resource)
  vi.spyOn(ledgerAPI,'renew').mockResolvedValue({}); vi.spyOn(ledgerAPI,'voidEntry').mockResolvedValue(entry); vi.spyOn(ledgerAPI,'history').mockResolvedValue([]); vi.spyOn(ledgerAPI,'saveSettings').mockResolvedValue(cfg.settings)
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.restoreAllMocks(); vi.useRealTimers() })

describe('Admin ledger', () => {
  it('preserves exact money and timezone conversions', () => {
    expect(money('1000000.123456')).toBe('1,000,000.123456'); expect(money('-0.000001')).toBe('-0.000001'); expect(money('5.000000')).toBe('5.00')
    const iso = '2026-10-23T02:15:00.000Z'; expect(isoDatetime(localDatetime(iso))).toBe(iso)
    expect(calendarDays('2026-03-08T13:00:00Z', new Date('2026-03-07T14:00:00Z'), 'America/New_York')).toBe(1)
  })
  it('shows independent cash and noncash totals without touching billing', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('.ledger-stat')).toHaveLength(4); expect(wrapper.text()).toContain('87.499999'); expect(wrapper.text()).toContain(en.ledger.giftHint)
    expect(wrapper.findAll('tbody tr')).toHaveLength(1); expect(ledgerAPI.saveEntry).not.toHaveBeenCalled()
  })
  it('clones into a new record and retries uncertain submissions using the same request key', async () => {
    vi.mocked(ledgerAPI.saveEntry).mockRejectedValueOnce({ status: 500 })
    const wrapper = render(); await flushPromises(); await button(wrapper,en.ledger.clone).trigger('click')
    await wrapper.get('#ledger-entry').trigger('submit'); await flushPromises(); const first = vi.mocked(ledgerAPI.saveEntry).mock.calls[0]!
    expect(first[0]).toBe(''); expect(first[1]).toMatchObject({ version: 0, occurred_on: '2026-10-23', resource_id: 'r1' })
    await wrapper.get('#ledger-entry').trigger('submit'); await flushPromises(); expect(vi.mocked(ledgerAPI.saveEntry).mock.calls[1]![2]).toBe(first[2]); expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
  it('generates a new key for edited payloads and recognizes normalized API conflicts', async () => {
    vi.mocked(ledgerAPI.saveEntry).mockRejectedValue({ status: 409, message: 'private detail' })
    const wrapper = render(); await flushPromises(); await button(wrapper,en.ledger.edit).trigger('click')
    await wrapper.get('#ledger-entry').trigger('submit'); await flushPromises(); expect(wrapper.get('[role="alert"]').text()).toBe(en.ledger.conflict)
    await wrapper.get('#ledger-entry textarea').setValue('Changed note'); await wrapper.get('#ledger-entry').trigger('submit'); await flushPromises()
    const calls = vi.mocked(ledgerAPI.saveEntry).mock.calls; expect(calls[0]![2]).not.toBe(calls[1]![2]); expect(wrapper.text()).not.toContain('private detail')
  })
  it('can renew date only without a duplicate expense and rejects non-advancing dates', async () => {
    const wrapper = render(); await flushPromises(); await wrapper.get('#ledger-tab-resources').trigger('click'); await button(wrapper,en.ledger.renew).trigger('click')
    await wrapper.get('#ledger-renew input[type="datetime-local"]').setValue(localDatetime(resource.expires_at)); await wrapper.get('#ledger-renew').trigger('submit'); await flushPromises(); expect(ledgerAPI.renew).not.toHaveBeenCalled()
    await wrapper.get('#ledger-renew input[type="datetime-local"]').setValue('2027-10-30T20:00'); await wrapper.get('#ledger-renew input[type="checkbox"]').setValue(false); await wrapper.get('#ledger-renew').trigger('submit'); await flushPromises()
    expect(ledgerAPI.renew).toHaveBeenCalledWith('r1',expect.objectContaining({ version: 3, record_expense: false }),expect.any(String))
  })
  it('keeps archived resources visible and never follows unsafe management links', async () => {
    vi.mocked(ledgerAPI.resources).mockResolvedValue({ items: [{ ...resource, active: false, management_url: 'javascript:alert(1)' }], timezone: 'Asia/Shanghai', server_time: '2026-10-23T02:00:00Z' })
    const wrapper = render(); await flushPromises(); await wrapper.get('#ledger-tab-resources').trigger('click'); expect(wrapper.findAll('.ledger-resource')).toHaveLength(0)
    await wrapper.get('#ledger-panel-resources select').setValue('archived'); expect(wrapper.findAll('.ledger-resource')).toHaveLength(1); expect(wrapper.find('a[target="_blank"]').exists()).toBe(false)
  })
  it('does not silently truncate expiry seconds when editing only a resource note', async () => {
    const exact = '2026-10-30T12:00:45.123Z'
    vi.mocked(ledgerAPI.resources).mockResolvedValue({ items: [{ ...resource, expires_at: exact }], timezone: 'Asia/Shanghai', server_time: '2026-10-23T02:00:00Z' })
    const wrapper = render(); await flushPromises(); await wrapper.get('#ledger-tab-resources').trigger('click'); await button(wrapper,en.ledger.edit).trigger('click')
    await wrapper.get('#ledger-resource textarea').setValue('New note'); await wrapper.get('#ledger-resource').trigger('submit'); await flushPromises()
    expect(ledgerAPI.saveResource).toHaveBeenCalledWith('r1',expect.objectContaining({ expires_at: exact, note: 'New note', version: 3 }),expect.any(String))
  })
  it('keeps email disabled by default and saves explicit administrator settings', async () => {
    const wrapper = render(); await flushPromises(); await wrapper.get(`button[aria-label="${en.ledger.settings}"]`).trigger('click'); await flushPromises()
    expect((wrapper.get('#ledger-settings input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('#ledger-settings input:not([type="checkbox"])').setValue('14, 3, 1'); await wrapper.get('#ledger-settings').trigger('submit'); await flushPromises()
    expect(ledgerAPI.saveSettings).toHaveBeenCalledWith({ ...cfg.settings, reminder_days: [14,3,1] })
  })
  it('shows settings load failures without spinning indefinitely or leaking private errors', async () => {
    const wrapper = render(); await flushPromises(); vi.mocked(ledgerAPI.settings).mockRejectedValue(new Error('private SMTP detail'))
    await wrapper.get(`button[aria-label="${en.ledger.settings}"]`).trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toBe(en.ledger.loadError); expect(wrapper.text()).not.toContain('private SMTP detail')
  })
  it('does not display blank cash totals when the only entries are credit gifts', async () => {
    vi.mocked(ledgerAPI.entries).mockResolvedValue({ ...result, totals: [{ currency: 'CREDIT', income: '0', expense: '0', net: '0', gift: '100' }] })
    const wrapper = render(); await flushPromises(); expect(wrapper.findAll('.ledger-stat dl')).toHaveLength(1)
    expect(wrapper.findAll('.ledger-stat')[0]!.text()).toContain('—'); expect(wrapper.findAll('.ledger-stat')[3]!.text()).toContain('100.00')
  })
  it('debounces filters and cancels outstanding loads on unmount', async () => {
    const wrapper = render(); await flushPromises(); await wrapper.get('input[type="month"]').setValue('2026-09'); await vi.advanceTimersByTimeAsync(300); await flushPromises()
    expect(ledgerAPI.entries).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2026-09', page: 1 }),expect.any(AbortSignal))
    const signal = vi.mocked(ledgerAPI.entries).mock.calls.at(-1)![1]!; wrapper.unmount(); expect(signal.aborted).toBe(true)
  })
})
