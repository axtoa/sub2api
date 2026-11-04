import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import Inspection from '../ChannelInspection.vue'
import { qualityAPI, type InspectionQuestion, type QualityRun } from '../api'
import { run, settings, target } from './fixtures'
import en from '@/i18n/locales/en/quality'

function lookup(key: string) { return key.split('.').reduce<unknown>((v, p) => v && typeof v === 'object' ? (v as Record<string, unknown>)[p] : undefined, en) }
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => { const value = lookup(key); return typeof value === 'string' ? value.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`)) : key }, te: (key: string) => typeof lookup(key) === 'string' }) }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn(), post: vi.fn() } }))
const report: QualityRun = { ...run, kind: 'inspection', inspection: { mode: 'quick', checks: ['tools'] }, verdict: 'passed', result: { ...run.result!, checks: [{ id: 'tools', verdict: 'passed', requests: 2, sample_ids: ['q1'], median_first_token_ms: null, tokens_per_second: null, cache_ratio: null }], items: run.result!.items!.map(item => ({ ...item, check_id: 'tools' })) } }
const wrappers: ReturnType<typeof mount>[] = []
function render(questions: InspectionQuestion[] = []) { const wrapper = mount(Inspection, { props: { targets: [target], settings: { ...settings, inspection_questions: questions } } }); wrappers.push(wrapper); return wrapper }
beforeEach(() => {
  vi.useFakeTimers(); vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  vi.spyOn(qualityAPI, 'inspectionQuestions').mockResolvedValue({ items: [], version: 1 })
  vi.spyOn(qualityAPI, 'history').mockResolvedValue([])
  vi.spyOn(qualityAPI, 'detail').mockResolvedValue(report)
  vi.spyOn(qualityAPI, 'inspect').mockResolvedValue({ ...report, status: 'queued' })
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.restoreAllMocks(); vi.useRealTimers() })

describe('Channel inspection', () => {
  it('opens without paid requests and displays preset costs before submission', async () => {
    const wrapper = render(); await flushPromises()
    expect(qualityAPI.inspect).not.toHaveBeenCalled()
    expect(wrapper.findAll('input:checked')).toHaveLength(6)
    expect(wrapper.text()).toContain('Up to 9 paid requests')
    await wrapper.findAll('select')[1]!.setValue('full')
    expect(wrapper.findAll('input:checked')).toHaveLength(9)
    expect(wrapper.text()).toContain('Up to 19 paid requests')
  })
  it('submits selected checks, follows persisted jobs, and stops polling on close', async () => {
    const wrapper = render(); await flushPromises()
    vi.mocked(qualityAPI.history).mockResolvedValue([{ ...report, status: 'queued' }])
    vi.mocked(qualityAPI.detail).mockResolvedValue({ ...report, status: 'queued' })
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(qualityAPI.inspect).toHaveBeenCalledWith(target.id, { mode: 'quick', checks: ['protocol_stream', 'protocol_nonstream', 'tools', 'structured', 'reasoning', 'speed'] })
    expect(wrapper.text()).toContain(en.quality.inspector.pending)
    const calls = vi.mocked(qualityAPI.history).mock.calls.length
    await vi.advanceTimersByTimeAsync(3000); expect(qualityAPI.history).toHaveBeenCalledTimes(calls + 1)
    wrapper.unmount(); const after = vi.mocked(qualityAPI.history).mock.calls.length
    await vi.advanceTimersByTimeAsync(60000); expect(qualityAPI.history).toHaveBeenCalledTimes(after)
  })
  it('restores reports and shows evidence as text without claiming model identity', async () => {
    vi.mocked(qualityAPI.history).mockResolvedValue([report])
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain(en.quality.inspector.checks.tools)
    expect(wrapper.text()).toContain(en.quality.inspector.identityHint)
    expect(wrapper.text()).toContain('Count the paths. Return JSON.')
    expect(wrapper.findAll('.quality-inspection-result')).toHaveLength(1)
    expect(qualityAPI.history).toHaveBeenCalledWith(target.id, 'inspection', expect.any(AbortSignal))
  })
  it('does not automatically replay an unconfirmed paid submission', async () => {
    vi.mocked(qualityAPI.inspect).mockRejectedValue(new Error('private upstream credentials'))
    const wrapper = render(); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe(en.quality.inspector.submitError)
    await vi.advanceTimersByTimeAsync(10000)
    expect(qualityAPI.inspect).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('private upstream credentials')
  })

  it('selects a subset, searches without losing selection, and snapshots content independently', async () => {
    const questions: InspectionQuestion[] = [{ name: 'My task', prompt: 'Explain caching', match: 'contains', expected: 'cache\nhit' }, { name: 'Other task', prompt: '2+2?', match: 'exact', expected: '4' }]
    const items = questions.map((q, index) => ({ ...q, id: `question-${index}` }))
    vi.mocked(qualityAPI.inspectionQuestions).mockResolvedValue({ items, version: 1 })
    const wrapper = render(); await flushPromises()
    for (const input of wrapper.findAll('.quality-inspection-checks input')) await input.setValue(false)
    await wrapper.get('.quality-custom-entry input[type="checkbox"]').setValue(true); await flushPromises()
    await wrapper.findAll('.quality-library-choice input')[0]!.setValue(true)
    await wrapper.get('.quality-library-picker input:not([type="checkbox"])').setValue('Other')
    expect(wrapper.findAll('.quality-library-choice')).toHaveLength(1)
    expect(wrapper.get('.quality-library-selected').text()).toContain('My task')
    expect(wrapper.text()).toContain('Up to 1 paid requests')
    items[0]!.name = 'Changed in library'
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(qualityAPI.inspect).toHaveBeenCalledWith(target.id, { mode: 'quick', checks: ['custom'], questions: [questions[0]] })
  })

  it('shares the eight-question limit between library selections and temporary questions', async () => {
    vi.mocked(qualityAPI.inspectionQuestions).mockResolvedValue({ items: Array.from({length: 9}, (_, i) => ({ id: `q-${i}`, name: `Task ${i}`, prompt: 'Explain caching', match: 'manual' as const, expected: '' })), version: 1 })
    const wrapper = render(); await flushPromises()
    await wrapper.get('.quality-custom-entry input[type="checkbox"]').setValue(true); await flushPromises()
    const add = wrapper.findAll('button').find(button => button.text() === en.quality.addQuestion)!
    await add.trigger('click')
    const inputs = wrapper.findAll('.quality-library-choice input')
    for (const input of inputs.slice(0, 7)) await input.setValue(true)
    expect(inputs[7]!.attributes('disabled')).toBeDefined()
    expect(add.attributes('disabled')).toBeDefined()
    await inputs[0]!.setValue(false)
    expect(inputs[7]!.attributes('disabled')).toBeUndefined()
    await wrapper.findAll('.quality-library-selected button')[0]!.trigger('click')
    expect(wrapper.findAll('.quality-library-selected button')).toHaveLength(5)
  })

  it('requires question fields and caps the custom editor at eight questions', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('.quality-custom-entry input[type="checkbox"]').setValue(true)
    const add = wrapper.findAll('button').find(button => button.text() === en.quality.addQuestion)!
    await add.trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(qualityAPI.inspect).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toBe(en.quality.custom.invalid)
    for (let i = 1; i < 8; i++) await add.trigger('click')
    expect(wrapper.findAll('.quality-custom-question')).toHaveLength(8)
    expect(add.attributes('disabled')).toBeDefined()
  })

  it('shows custom grading and manual review evidence without rendering model HTML', async () => {
    const custom: QualityRun = { ...report, result: { ...report.result!, checks: [{ ...report.result!.checks![0], id: 'custom', verdict: 'unknown', reason: 'manual_review' }], items: [{ ...report.result!.items![0], check_id: 'custom', name: 'Open-ended task', match: 'manual', grade_reason: 'manual_review', correct: null, probe: { ...report.result!.items![0].probe, text: '<script>bad()</script>' } }] } }
    vi.mocked(qualityAPI.history).mockResolvedValue([custom])
    vi.mocked(qualityAPI.detail).mockResolvedValue(custom)
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('Open-ended task')
    expect(wrapper.text()).toContain(en.quality.custom.review)
    expect(wrapper.text()).toContain('<script>bad()</script>')
    expect(wrapper.find('script').exists()).toBe(false)
  })

  it('does not present an incomplete automatic question as ready for manual review', async () => {
    const incomplete: QualityRun = { ...report, result: { ...report.result!, checks: [{ ...report.result!.checks![0], id: 'custom', verdict: 'uncompleted', reason: 'truncated' }], items: [{ ...report.result!.items![0], check_id: 'custom', name: 'Truncated task', match: 'exact', correct: null, probe: { ...report.result!.items![0].probe, complete: false, reason: 'truncated' } }] } }
    vi.mocked(qualityAPI.history).mockResolvedValue([incomplete])
    vi.mocked(qualityAPI.detail).mockResolvedValue(incomplete)
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('.quality-protocol summary').text()).toContain(en.quality.ungraded)
    expect(wrapper.get('.quality-protocol summary').text()).not.toContain(en.quality.custom.review)
  })
})
