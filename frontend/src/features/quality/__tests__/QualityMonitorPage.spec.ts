import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import QualityMonitorPage from '../QualityMonitorPage.vue'
import { attention, profileOf, qualityAPI } from '../api'
import type { QualityProfile } from '../api'
import { cachePercent, probeRate, probeSlots } from '../display'
import { account, run, settings, snapshot, target } from './fixtures'
import en from '@/i18n/locales/en/quality'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key:string, params:Record<string,unknown>={}) => { const value=key.split('.').reduce<unknown>((v,p)=>v && typeof v==='object' ? (v as Record<string,unknown>)[p] : undefined,en);return typeof value==='string' ? value.replace(/\{(\w+)\}/g,(_,name)=>String(params[name] ?? `{${name}}`)) : key },te:(key:string)=>typeof key.split('.').reduce<unknown>((v,p)=>v && typeof v==='object' ? (v as Record<string,unknown>)[p] : undefined,en)==='string' }) }))
vi.mock('@/api/client',()=>({apiClient:{get:vi.fn(),post:vi.fn(),put:vi.fn()}}))
vi.mock('@/components/layout/AppLayout.vue',()=>({default:defineComponent({template:'<main><slot /></main>'})}))
vi.mock('@/components/common/BaseDialog.vue',()=>({default:defineComponent({props:['show','title'],template:'<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>'})}))
vi.mock('@/components/icons/Icon.vue',()=>({default:defineComponent({template:'<span />'})}))
const wrappers:Array<ReturnType<typeof mount>>=[]
function render(){const wrapper=mount(QualityMonitorPage);wrappers.push(wrapper);return wrapper}
function buttonByText(wrapper:ReturnType<typeof mount>,label:string){const button=wrapper.findAll('button').find(b=>b.text()===label);expect(button,label).toBeDefined();return button!}
beforeEach(()=>{
  vi.useFakeTimers();localStorage.clear();vi.spyOn(document,'hidden','get').mockReturnValue(false)
  vi.spyOn(qualityAPI,'sync').mockResolvedValue({created:0})
  vi.spyOn(qualityAPI,'snapshot').mockResolvedValue(structuredClone(snapshot))
  vi.spyOn(qualityAPI,'saveSettings').mockResolvedValue(settings)
  vi.spyOn(qualityAPI,'saveTarget').mockResolvedValue(target)
  vi.spyOn(qualityAPI,'saveAccount').mockResolvedValue(undefined)
  vi.spyOn(qualityAPI,'history').mockResolvedValue([run]);vi.spyOn(qualityAPI,'detail').mockResolvedValue(run)
  let next=0;vi.spyOn(qualityAPI,'run').mockImplementation(async()=>({queued:1,skipped:0,jobs:[{...run,id:`queued-${++next}`,status:'queued'}]}))
})
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());vi.restoreAllMocks();vi.useRealTimers()})

describe('Quality account monitor',()=>{
  it('initially syncs once, renders all accounts including paused and unsupported, without duplicate model cards',async()=>{
    vi.mocked(qualityAPI.snapshot).mockResolvedValue({...structuredClone(snapshot),accounts:[account,{...account,id:2,name:'Paused',enabled:false},{...account,id:3,name:'Unsupported',unsupported:true}],targets:[target,{...target,id:'watch-2',model:'another-model'}]})
    const wrapper=render();await flushPromises()
    expect(qualityAPI.sync).toHaveBeenCalledWith(true);expect(wrapper.findAll('.quality-account-card')).toHaveLength(3)
    expect(wrapper.findAll('.quality-account-card')[0]!.findAll('.quality-mark')).toHaveLength(180)
    expect(wrapper.findAll('.quality-account-card')[2]!.get('.quality-card-actions button').attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(60000);expect(qualityAPI.sync).toHaveBeenCalledTimes(1)
    await buttonByText(wrapper,en.quality.sync).trigger('click');await flushPromises();expect(qualityAPI.sync).toHaveBeenLastCalledWith(false)
  })
  it('switches density without fetching or changing model settings and remembers the choice',async()=>{
    const wrapper=render();await flushPromises();expect(wrapper.get('[data-density]').attributes('data-density')).toBe('compact')
    await buttonByText(wrapper,en.quality.standard).trigger('click');expect(wrapper.get('[data-density]').attributes('data-density')).toBe('standard')
    expect(localStorage.getItem('best-ai.quality.density')).toBe('standard');expect(qualityAPI.snapshot).toHaveBeenCalledTimes(1);expect(qualityAPI.saveTarget).not.toHaveBeenCalled()
  })
  it('keeps a multi-group account unique in group search and filtering',async()=>{
    const wrapper=render();await flushPromises();await wrapper.get('.quality-group-popover input').setValue('Second')
    expect(wrapper.findAll('.quality-group-options button')).toHaveLength(1);await wrapper.get('.quality-group-options button').trigger('click')
    expect(wrapper.findAll('.quality-account-card')).toHaveLength(1);expect(wrapper.get('.quality-group-picker summary').text()).toContain('Second group')
  })
  it('submits every manual click even when account/target automation is off, budget is exhausted and work is pending',async()=>{
    vi.mocked(qualityAPI.snapshot).mockResolvedValue({...structuredClone(snapshot),settings:{...settings,auto_enabled:false},accounts:[{...account,enabled:false}],budget:{...snapshot.budget,limited:true},targets:[{...target,enabled:false,pending:{...run,status:'queued'}}]})
    const wrapper=render();await flushPromises();const button=wrapper.get('.quality-card-actions button:last-child')
    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click');await button.trigger('click');await button.trigger('click');await flushPromises()
    expect(qualityAPI.run).toHaveBeenCalledTimes(3);expect(qualityAPI.run).toHaveBeenNthCalledWith(1,['watch-1'],'quality')
  })
  it('only changes monitoring preferences when toggling an account',async()=>{
    const wrapper=render();await flushPromises();await wrapper.get('.quality-auto input').setValue(false);await flushPromises()
    expect(qualityAPI.saveAccount).toHaveBeenCalledWith(expect.objectContaining({id:1,version:2}),false);expect(qualityAPI.saveTarget).not.toHaveBeenCalled()
  })
  it('preserves settings version while restoring defaults',async()=>{
    const wrapper=render();await flushPromises();await wrapper.get(`button[aria-label="${en.quality.settings}"]`).trigger('click')
    await buttonByText(wrapper,en.quality.defaults).trigger('click');await wrapper.get('#quality-settings').trigger('submit');await flushPromises()
    expect(qualityAPI.saveSettings).toHaveBeenCalledWith(expect.objectContaining({version:7,daily_requests:1000,cache_enabled:false}))
  })
  it('shows exact question, reference, raw answer and local explanation in the drawer',async()=>{
    const wrapper=render();await flushPromises();await wrapper.get('.quality-compact-facts button:last-child').trigger('click');await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('Count the paths. Return JSON.')
    expect(wrapper.get('[role="dialog"]').text()).toContain('{"answer":8}');expect(wrapper.get('[role="dialog"]').text()).toContain('{"answer":12}')
    expect(wrapper.text()).toContain('20 minus 12 equals 8.')
  })
  it('does not show an interrupted fully graded round as a pass, or expose unknown question translation keys',async()=>{
    const interrupted={...run,verdict:'unknown',result:{...run.result!,reason:'configuration_changed',items:run.result!.items!.map(item=>({...item,family:'legacy-family'}))}}
    vi.mocked(qualityAPI.snapshot).mockResolvedValue({...structuredClone(snapshot),targets:[{...target,latest_quality:interrupted}]})
    vi.mocked(qualityAPI.detail).mockResolvedValue(interrupted)
    const wrapper=render();await flushPromises()
    expect(wrapper.get('.quality-compact-facts button:last-child').text()).toBe(en.quality.incompleteShort)
    await wrapper.get('.quality-compact-facts button:last-child').trigger('click');await flushPromises()
    expect(wrapper.get('.quality-evidence h3').text()).toBe(en.quality.question)
    expect(wrapper.text()).not.toContain('quality.families.legacy-family')
  })
  it('loads older chart evidence directly even when outside the 60-item history page',async()=>{
    vi.mocked(qualityAPI.snapshot).mockResolvedValue({...structuredClone(snapshot),targets:[{...target,recent:[{...run,id:'older-probe',kind:'availability'}]}]})
    const wrapper=render();await flushPromises();await wrapper.get('.quality-mark:last-child').trigger('click');await flushPromises()
    expect(qualityAPI.detail).toHaveBeenCalledWith('older-probe')
  })
  it('keeps the selected historical evidence when results refresh',async()=>{
    const older={...run,id:'older-run'}
    vi.mocked(qualityAPI.history).mockResolvedValue([run,older])
    vi.mocked(qualityAPI.detail).mockImplementation(async id=>id==='older-run'?older:run)
    const wrapper=render();await flushPromises()
    await buttonByText(wrapper,en.quality.records).trigger('click');await flushPromises()
    await wrapper.findAll('.quality-history-row')[1]!.trigger('click');await flushPromises()
    expect(wrapper.findAll('.quality-history-row')[1]!.attributes('aria-pressed')).toBe('true')
    await vi.advanceTimersByTimeAsync(60000);await flushPromises()
    expect(qualityAPI.detail).toHaveBeenLastCalledWith('older-run')
    expect(wrapper.findAll('.quality-history-row')[1]!.attributes('aria-pressed')).toBe('true')
  })
  it('saves complete overrides or null inheritance, without leaking global budget fields',async()=>{
    const wrapper=render();await flushPromises();await wrapper.get(`button[aria-label="${en.quality.edit}"]`).trigger('click');await flushPromises()
    await wrapper.get('#quality-target').trigger('submit');await flushPromises();expect(qualityAPI.saveTarget).toHaveBeenLastCalledWith(expect.objectContaining({version:3}),null,true)
    await wrapper.findAll('#quality-target input[type="checkbox"]')[1]!.setValue(false);await wrapper.get('#quality-target').trigger('submit');await flushPromises()
    const profile=vi.mocked(qualityAPI.saveTarget).mock.calls.at(-1)![1] as QualityProfile;expect(profile.questions).toBe(3);expect(profile).not.toHaveProperty('daily_tokens')
  })
  it('stops polling on unmount and does not expose private errors',async()=>{
    const wrapper=render();await flushPromises();wrapper.unmount();const calls=vi.mocked(qualityAPI.snapshot).mock.calls.length;await vi.advanceTimersByTimeAsync(180000);expect(qualityAPI.snapshot).toHaveBeenCalledTimes(calls)
    vi.mocked(qualityAPI.sync).mockRejectedValue(new Error('private backend detail'));const failed=render();await flushPromises();expect(failed.get('[role="alert"]').text()).toBe(en.quality.loadError);expect(failed.text()).not.toContain('private backend detail')
  })
  it('distinguishes unavailable/missing history and cache fields from success or zero',()=>{
    expect(probeRate(target)).toBeNull();expect(probeRate({...target,probe_successes:3,probe_failures:1})).toBe(75)
    expect(probeSlots(target)).toHaveLength(180);expect(probeSlots(target).every(x=>x===null)).toBe(true)
    const cached={...run,kind:'cache' as const,verdict:'cache_hit',result:{...run.result!,cache_reported:true,cache_input_tokens:5000,cache_read_tokens:4608}}
    expect(cachePercent(run)).toBeNull();expect(cachePercent(cached)).toBeCloseTo(92.16)
    expect(cachePercent({...cached,verdict:'not_observed',result:{...cached.result,cache_read_tokens:0}})).toBe(0)
    expect(cachePercent({...cached,verdict:'unknown',result:{...cached.result,reason:'transport_error'}})).toBeNull()
    expect(profileOf(settings)).not.toHaveProperty('daily_tokens');expect(attention({...target,enabled:false,latest_quality:run})).toBe(true)
  })
})
