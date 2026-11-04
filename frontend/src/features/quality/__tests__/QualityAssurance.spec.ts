import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import QualityAssurance from '../QualityAssurance.vue'
import { qualityAPI, scoreDimensions, type AssuranceSettings, type ScoreBoard, type QualityScore } from '../api'
import { account, target, run } from './fixtures'
import en from '@/i18n/locales/en/quality'

function lookup(key: string) { return key.split('.').reduce<unknown>((v,p) => v && typeof v === 'object' ? (v as Record<string,unknown>)[p] : undefined,en) }
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t:(key:string,params:Record<string,unknown>={}) => {const value=lookup(key);return typeof value === 'string' ? value.replace(/\{(\w+)\}/g,(_,name)=>String(params[name] ?? '{'+name+'}')) : key},te:(key:string)=>typeof lookup(key)==='string' }) }))
vi.mock('@/api/client', () => ({apiClient:{get:vi.fn(),put:vi.fn(),post:vi.fn()}}))
const cfg: AssuranceSettings={version:1,references:[],manual_references:[],policy:{scenario:'chat',weights:{availability:25,capability:25,compatibility:20,latency:10,integrity:10,billing:10},min_samples:8,min_runs:3,min_span_minutes:60,billing_tolerance_percent:2,min_coverage:60,fresh_hours:48,slow_ms:5000,failure_percent:20,usage_tolerance_percent:15,required_checks:['protocol_stream']}}
const row: QualityScore={target_id:target.id,account_id:account.id,account_name:account.name,platform:'openai',model:target.model,cohort:'openai:model:abc',score:78.3,coverage:80,samples:40,runs:4,excluded:2,truncated:false,rank:1,state:'usable',last_at:'2026-11-04T00:00:00Z',dimensions:scoreDimensions.map(id=>({id,value:id==='integrity'||id==='billing'?null:78,samples:10,passed:10,p95_ms:null})),issues:[],evidence:[],run_ids:[run.id]}
const board: ScoreBoard={version:'assurance-v1',generated_at:'2026-11-04T00:00:00Z',days:7,policy:cfg.policy,items:[row]}
const wrappers: ReturnType<typeof mount>[]=[]
function render(){const w=mount(QualityAssurance,{props:{targets:[target],accounts:[account]},global:{stubs:{BaseDialog:{props:['show'],template:'<div v-if="show"><slot /></div>'}}}});wrappers.push(w);return w}
function button(w:ReturnType<typeof mount>,text:string){return w.findAll('button').find(b=>b.text()===text)!}
beforeEach(()=>{
 vi.useFakeTimers();vi.spyOn(document,'hidden','get').mockReturnValue(false)
 vi.spyOn(qualityAPI,'assurance').mockResolvedValue(JSON.parse(JSON.stringify(cfg)))
 vi.spyOn(qualityAPI,'scores').mockResolvedValue(JSON.parse(JSON.stringify(board)))
 vi.spyOn(qualityAPI,'saveAssurance').mockImplementation(async value=>JSON.parse(JSON.stringify({...value,version:value.version+1})))
 vi.spyOn(qualityAPI,'inspect').mockResolvedValue({...run,kind:'inspection',status:'queued'})
 vi.spyOn(qualityAPI,'statuses').mockResolvedValue([{...run,kind:'inspection',status:'finished'}])
 vi.spyOn(qualityAPI,'detail').mockResolvedValue(run)
 vi.spyOn(qualityAPI,'saveEvidence').mockImplementation(async value=>value)
})
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());vi.restoreAllMocks();vi.useRealTimers()})
describe('Upstream quality assessment',()=>{
 it('shows measured coverage and unknown dimensions without starting probes',async()=>{
  const w=render();await flushPromises()
  expect(w.text()).toContain('78.3');expect(w.text()).toContain('80% evidence coverage')
  expect(w.findAll('.quality-score-dimensions strong').map(node=>node.text()).slice(-2)).toEqual(['—','—'])
  expect(qualityAPI.inspect).not.toHaveBeenCalled()
  expect(w.text()).toContain(en.quality.assurance.boundaries)
 })
 it('keeps critical, unranked and untested accounts visible',async()=>{
  vi.mocked(qualityAPI.scores).mockResolvedValue({...board,items:[{...row,score:95,rank:null,state:'blocked',issues:[{code:'privacy',severity:'block',run_ids:[run.id]}]}]})
  const w=render();await flushPromises()
  expect(w.text()).toContain('Critical issue')
  expect(w.text()).toContain('Data privacy verification record')
  expect(w.get('.quality-score-card').text()).not.toContain('#1')
 })
 it('saves references as account/model IDs and preserves unsaved edits after a conflict',async()=>{
  const w=render();await flushPromises();await button(w,en.quality.assurance.rules).trigger('click')
  await button(w,en.quality.assurance.addReference).trigger('click')
  const row=w.get('.quality-reference-row')
  await row.get('input').setValue('Official key account')
  await row.findAll('select')[0]!.setValue(target.id)
  await row.findAll('select')[1]!.setValue('official')
  vi.mocked(qualityAPI.saveAssurance).mockRejectedValueOnce(new Error('conflict'))
  await w.get('form').trigger('submit');await flushPromises()
  expect(w.text()).toContain(en.quality.assurance.saveError)
  expect((row.get('input').element as HTMLInputElement).value).toBe('Official key account')
  expect(qualityAPI.saveAssurance).toHaveBeenCalledWith(expect.objectContaining({references:[expect.objectContaining({target_id:target.id,source:'official'})]}))
  expect(JSON.stringify(vi.mocked(qualityAPI.saveAssurance).mock.calls)).not.toContain('api_key')
  expect(qualityAPI.inspect).not.toHaveBeenCalled()
 })
 it('requires a model cohort and explicit confirmation before batch paid probes',async()=>{
  const w=render();await flushPromises()
  const form=w.get('.quality-assessment-batch form')
  expect(form.get('button.primary').attributes('disabled')).toBeDefined()
  await w.findAll('select')[2]!.setValue(row.cohort)
  await button(w,en.quality.assurance.selectAll).trigger('click')
  expect(form.text()).toContain('1 accounts selected; up to 8 paid requests.')
  expect(qualityAPI.inspect).not.toHaveBeenCalled()
  await form.trigger('submit');await flushPromises()
  expect(qualityAPI.inspect).toHaveBeenCalledTimes(1)
  expect(qualityAPI.inspect).toHaveBeenCalledWith(target.id,{mode:'quick',checks:['protocol_stream','reasoning','speed','cache']})
  await vi.advanceTimersByTimeAsync(3000);expect(qualityAPI.statuses).toHaveBeenCalled()
 })
 it('does not retry an unconfirmed submission or poll after unmount',async()=>{
  vi.mocked(qualityAPI.inspect).mockRejectedValueOnce(new Error('unknown'))
  const w=render();await flushPromises();await w.findAll('select')[2]!.setValue(row.cohort);await button(w,en.quality.assurance.selectAll).trigger('click')
  await w.get('.quality-assessment-batch form').trigger('submit');await flushPromises()
  await vi.advanceTimersByTimeAsync(10000);expect(qualityAPI.inspect).toHaveBeenCalledTimes(1)
  w.unmount();const calls=vi.mocked(qualityAPI.scores).mock.calls.length
  await vi.advanceTimersByTimeAsync(120000);expect(qualityAPI.scores).toHaveBeenCalledTimes(calls)
 })
 it('appends attributable billing evidence to the selected run',async()=>{
  const w=render();await flushPromises()
  await button(w,'#run-1').trigger('click');await flushPromises()
  const form=w.findAll('form').at(-1)!
  const inputs=form.findAll('input')
  await inputs[0]!.setValue('ledger-123')
  await inputs[2]!.setValue('1')
  await inputs[3]!.setValue('1.5')
  await form.get('textarea').setValue('Request-level ledger checked')
  await form.get('input[type="checkbox"]').setValue(true)
  await form.trigger('submit');await flushPromises()
  expect(qualityAPI.saveEvidence).toHaveBeenCalledWith(expect.objectContaining({run_id:run.id,receipt:'ledger-123',expected:1,actual:1.5,attributable:true}))
 })
})
